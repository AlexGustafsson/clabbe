package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/openai"
	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/AlexGustafsson/clabbe/internal/streaming"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
	"github.com/AlexGustafsson/clabbe/internal/webm"
)

var (
	ErrNoStreamPlaying = errors.New("no stream is playing")
)

type Bot struct {
	state *state.State

	openai *openai.Client

	mutex         sync.Mutex
	shouldPlay    bool
	currentEntry  *state.PlaylistEntry
	currentStream streaming.AudioStream
}

func New(state *state.State, openai *openai.Client) *Bot {
	return &Bot{
		state: state,

		openai: openai,
	}
}

// Search performs a search for content.
// Returns the first result. When using AI, returns the first result for each
// additional query provided by the AI.
func (b *Bot) Search(ctx context.Context, query string, useAI bool) ([]youtube.SearchResult, error) {
	slog.Debug("Performing search", slog.String("query", query), slog.Bool("useAi", useAI))
	queries := make([]string, 0)

	if useAI && b.openai != nil {
		slog.Debug("Extrapolating search using AI", slog.String("query", query))
		res, err := b.openai.FetchCompletion(ctx, &openai.CompletionRequest{
			Messages: []openai.Message{
				{
					Role:    openai.RoleSystem,
					Content: b.state.Config.Prompt,
				},
				{
					Role:    openai.RoleUser,
					Content: query,
				},
			},
			Temperature:      1,
			MaxTokens:        256,
			TopP:             1,
			FrequencyPenalty: 0,
			PresencePenalty:  0,
			Model:            openai.DefaultModel,
			Stream:           false,
		})
		if err != nil {
			return nil, err
		}
		b.state.Metrics.TokensConsumed.Add(float64(res.Usage.TotalTokens))

		if len(res.Choices) > 0 {
			response := res.Choices[0].Message.Content
			if response == "no results" {
				slog.Debug("No results from LLM")
				return []youtube.SearchResult{}, nil
			}
			slog.Debug("Got response from AI", slog.String("response", response))

			// TODO: Assume default prompt for now
			entries := strings.Split(response, "\n")
			for _, entry := range entries {
				// The default output has indexes, remove them
				_, query, _ := strings.Cut(entry, " ")
				queries = append(queries, query)
			}
		} else {
			slog.Debug("No response from LLM")
			return []youtube.SearchResult{}, nil
		}
	} else {
		// Use query verbatim as use of AI was not requested
		queries = []string{query}
	}

	slog.Debug("Searching for results on YouTube", slog.Any("queries", queries))

	allResults := make([]youtube.SearchResult, 0)
	for _, query := range queries {
		results, err := youtube.Search(ctx, query)
		if err != nil {
			return nil, err
		}

		if len(results) > 0 {
			allResults = append(allResults, results[0])
		}
	}

	return allResults, nil
}

type QueueOptions struct {
	// UseAI defaults to false.
	UseAI bool
}

// Queue performs a search for content and adds the top result to the playlist.
func (b *Bot) Queue(ctx context.Context, query string, addedBy state.Entity, options *QueueOptions) ([]state.PlaylistEntry, error) {
	slog.Debug("Queueing", slog.String("query", query))
	if options == nil {
		options = &QueueOptions{}
	}

	useAI := options.UseAI

	results, err := b.Search(ctx, query, useAI)
	if err != nil {
		return nil, err
	}

	entries := make([]state.PlaylistEntry, len(results))

	if len(results) > 0 {
		slog.Debug("Got results to queue", slog.Any("results", results))
		b.mutex.Lock()
		for i, result := range results {
			entry := state.PlaylistEntry{
				Time:    time.Now(),
				Title:   result.Title,
				AddedBy: addedBy,
				Source:  state.SourceYouTube,
				URI:     result.ID,
			}
			entries[i] = entry
			b.state.Queue.AddEntry(entry)
		}
		b.mutex.Unlock()
	} else {
		slog.Debug("No results")
	}

	return entries, nil
}

type SuggestOptions struct {
	// UseAI defaults to false.
	UseAI bool
}

// Suggest adds the results as a basis for songs to play when interpolating.
func (b *Bot) Suggest(ctx context.Context, addedBy state.Entity, query string) ([]state.PlaylistEntry, error) {
	slog.Debug("Adding suggestions", slog.String("query", query))
	results, err := b.Search(ctx, query, true)
	if err != nil {
		return nil, err
	}

	entries := make([]state.PlaylistEntry, len(results))

	slog.Debug("Got results to add to suggestions", slog.Any("results", results))
	b.mutex.Lock()
	for i, result := range results {
		entry := state.PlaylistEntry{
			Time:    time.Now(),
			Title:   result.Title,
			AddedBy: addedBy,
			Source:  state.SourceYouTube,
			URI:     result.ID,
		}
		entries[i] = entry
		b.state.Suggestions.AddEntry(entry)
	}
	b.mutex.Unlock()

	return entries, nil
}

// Extrapolate adds some entries to the playlist based on suggestions and
// history.
func (b *Bot) Extrapolate(ctx context.Context) error {
	// If possible, use the suggestions immediately
	b.mutex.Lock()
	suggestions := b.state.Suggestions.PopN(5)
	if len(suggestions) > 0 {
		slog.Debug("There were unused suggestions, using them first")
		for _, suggestion := range suggestions {
			suggestion.AddedBy = state.Entity{
				Role: state.RoleSystem,
			}
			b.state.Queue.Push(suggestion)
		}
		b.mutex.Unlock()
		return nil
	}

	// If no suggestions were ready, extrapolate based on history
	if b.openai == nil {
		b.mutex.Unlock()
		return fmt.Errorf("missing required Open AI client")
	}

	var lookback strings.Builder
	entries := b.state.History.PeakBackN(b.state.Config.ExtrapolationLookback)
	for i, entry := range entries {
		fmt.Fprintf(&lookback, "%d. %s\n", i+1, entry.Title)
	}
	b.mutex.Unlock()

	slog.Debug("Extrapolating songs based on history", slog.String("history", lookback.String()))
	res, err := b.openai.FetchCompletion(ctx, &openai.CompletionRequest{
		Messages: []openai.Message{
			{
				Role:    openai.RoleSystem,
				Content: b.state.Config.Prompt,
			},
			{
				Role:    openai.RoleAssistant,
				Content: lookback.String(),
			},
			{
				Role:    openai.RoleUser,
				Content: "provide similar songs",
			},
		},
		Temperature:      1,
		MaxTokens:        256,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Model:            openai.DefaultModel,
		Stream:           false,
	})
	if err != nil {
		return err
	}
	b.state.Metrics.TokensConsumed.Add(float64(res.Usage.TotalTokens))

	if len(res.Choices) == 0 {
		slog.Debug("No response from LLM")
		return nil
	}

	response := res.Choices[0].Message.Content
	if response == "no results" {
		slog.Debug("No results from LLM")
		return nil
	}
	slog.Debug("Got response from Open AI", slog.String("response", response))

	// TODO: Assume default prompt for now
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		_, query, _ := strings.Cut(line, " ")
		entity := state.Entity{
			Role: state.RoleSystem,
		}
		if _, err := b.Queue(ctx, strings.TrimSpace(query), entity, nil); err != nil {
			return err
		}
	}

	return nil
}

// Play starts playing content, sending windows of OPUS-encoded audio to the
// provided channel.
func (b *Bot) Play(opus chan<- []byte, songs chan<- string) error {
	if b.currentStream != nil {
		return fmt.Errorf("already playing")
	}

	b.shouldPlay = true

	failures := 0

	for failures < 5 && b.shouldPlay {
		b.mutex.Lock()
		entry, ok := b.state.Queue.Pop()
		b.mutex.Unlock()

		if !ok {
			if b.state.Config.ExtrapolateWhenEmpty {
				slog.Debug("Playlist is empty, interpolating")
				err := b.Extrapolate(context.Background())
				if err != nil {
					return err
				}
				continue
			} else {
				slog.Debug("Playlist is empty, closing")
				return nil
			}
		}

		songs <- entry.Title
		err := b.playOnce(entry, opus)
		if err == nil {
			failures = 0
		} else {
			slog.Error("Failed to play entry", slog.Any("error", err))
			// Try next
			failures++
		}
	}

	return fmt.Errorf("too many errors")
}

// playOnce plays the entry, sending windows of OPUS-encoded audio to the
// provided channel.
func (b *Bot) playOnce(entry state.PlaylistEntry, opus chan<- []byte) error {
	slog.Debug("Playing", slog.String("uri", entry.URI), slog.String("title", entry.Title), slog.String("source", string(entry.Source)))
	defer func() {
		b.currentEntry = nil
		b.currentStream = nil
	}()

	// For now, assume YouTube as source
	b.mutex.Lock()
	stream, err := youtube.NewAudioStream(context.Background(), entry.URI, nil)
	if err != nil {
		slog.Error("Failed to create YouTube audio stream", slog.Any("error", err))
		b.mutex.Unlock()
		return err
	}
	defer stream.Close()

	if !strings.EqualFold(stream.MimeType(), `audio/webm; codecs="opus"`) {
		// For now, don't support other formats as they would need to be processed
		// by ffmpeg
		return fmt.Errorf("unsupported codec: %s", stream.MimeType())
	}

	webmReader := webm.NewReader(stream)

	b.currentEntry = &entry
	b.currentStream = stream
	b.state.History.AddEntry(entry)
	b.mutex.Unlock()

	b.state.Metrics.SongsPlayed.Inc()
	b.state.Metrics.ActiveStreams.Inc()
	playbackStarted := time.Now()
	defer func() {
		b.state.Metrics.DurationPlayed.Add(time.Since(playbackStarted).Seconds())
		b.state.Metrics.ActiveStreams.Dec()
	}()

	for {
		frame, err := webmReader.Read()
		if err == io.EOF {
			slog.Debug("Stream ended")
			break
		} else if err == io.ErrClosedPipe {
			slog.Debug("Stream closed")
			break
		} else if err != nil {
			slog.Error("Failed to read webm OPUS frame", slog.Any("error", err))
			return err
		}

		opus <- frame.Payload
	}

	slog.Debug("Stopped playing stream", slog.String("source", string(entry.Source)), slog.String("uri", entry.URI), slog.String("title", stream.Title()))
	return nil
}

// ClearPlaylist clears all entries of the playlist.
func (b *Bot) ClearPlaylist() {
	slog.Debug("Clearing playlist")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.state.Queue.Clear()
}

// ClearSuggestions clears all suggestions.
func (b *Bot) ClearSuggestions() {
	slog.Debug("Clearing suggestions")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.state.Suggestions.Clear()
}

// Stop stops the currently playing stream.
func (b *Bot) Stop() {
	slog.Debug("Stopping playing stream")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.shouldPlay = false
	// b.ClearPlaylist()
	// b.ClearSuggestions()
	if b.currentStream != nil {
		b.currentStream.Close()
	}
}

// Skip stops the currently playing stream.
func (b *Bot) Skip() {
	b.SkipN(0)
}

// SkipN skips n songs.
func (b *Bot) SkipN(n int) {
	if n <= 0 {
		n = 1
	}
	slog.Debug("Skipping playing stream(s)", slog.Int("n", n))
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.currentStream != nil {
		b.state.Queue.PopN(n - 1)
		b.currentStream.Close()
	}
}

// NowPlaying returns the current playlist entry, or nil if nothing is playing.
func (b *Bot) NowPlaying() *state.PlaylistEntry {
	return b.currentEntry
}

func (b *Bot) OpenAIEnabled() bool {
	return b.openai != nil
}
