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

	"github.com/AlexGustafsson/clabbe/internal/llm"
	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/AlexGustafsson/clabbe/internal/webm"
	"github.com/AlexGustafsson/clabbe/internal/youtube"
	"github.com/AlexGustafsson/clabbe/internal/ytdlp"
)

var (
	ErrNoStreamPlaying       = errors.New("no stream is playing")
	ErrUnsupportedAudioCodec = errors.New("unsupported audio codec")
)

type ExtrapolationType int

const (
	ExtrapolationTypeNone ExtrapolationType = iota << 1
	ExtrapolationTypeHistory
	ExtrapolationTypeSuggest
)

type Bot struct {
	ExtrapolationType ExtrapolationType

	state *state.State

	llm llm.Client

	mutex        sync.Mutex
	shouldPlay   bool
	currentEntry *state.PlaylistEntry

	isStreaming  bool
	cancelStream context.CancelFunc
}

func New(state *state.State, llm llm.Client) *Bot {
	extrapolationType := ExtrapolationTypeNone
	if state.Config.ExtrapolateWhenEmpty {
		extrapolationType = ExtrapolationTypeHistory
	}

	return &Bot{
		ExtrapolationType: extrapolationType,

		state: state,

		llm: llm,
	}
}

// Search performs a search for content.
// Returns the first result. When using AI, returns the first result for each
// additional query provided by the AI.
func (b *Bot) Search(ctx context.Context, query string, useAI bool) ([]youtube.SearchResult, error) {
	slog.Debug("Performing search", slog.String("query", query), slog.Bool("useAi", useAI))
	queries := make([]string, 0)

	if useAI && b.llm != nil {
		slog.Debug("Extrapolating search using AI", slog.String("query", query))
		res, err := b.llm.Chat(ctx, &llm.ChatRequest{
			Messages: []llm.Message{
				{
					Role:    llm.RoleSystem,
					Content: b.state.Config.Prompt,
				},
				{
					Role:    llm.RoleUser,
					Content: query,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		if len(res.Message.Content) == 0 {
			slog.Debug("No response from LLM")
			return []youtube.SearchResult{}, nil
		}

		if res.Message.Content == "no results" {
			slog.Debug("No results from LLM")
			return []youtube.SearchResult{}, nil
		}
		slog.Debug("Got response from AI", slog.String("response", res.Message.Content))

		// TODO: Assume default prompt for now
		entries := strings.Split(res.Message.Content, "\n")
		for _, entry := range entries {
			// The default output has indexes, remove them
			_, query, _ := strings.Cut(entry, " ")
			queries = append(queries, query)
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

	// If no suggestions were ready, extrapolate using AI
	if b.llm == nil {
		b.mutex.Unlock()
		return fmt.Errorf("missing required Open AI client")
	}

	// If auto play is on, suggest themes to itself
	if b.ExtrapolationType == ExtrapolationTypeSuggest {
		b.mutex.Unlock()
		return b.extrapolateWithThemeSuggestions(ctx)
	} else {
		return b.extrapolateWithHistory(ctx)
	}
}

func (b *Bot) extrapolateWithThemeSuggestions(ctx context.Context) error {
	slog.Debug("Extrapolating songs based on suggestions of new themes")
	res, err := b.llm.Chat(ctx, NewThemeRequest())
	if err != nil {
		return err
	}

	if len(res.Message.Content) == 0 {
		slog.Debug("No response from LLM")
		return nil
	}

	response := res.Message.Content
	if response == "no results" {
		slog.Debug("No results from LLM")
		return nil
	}
	slog.Debug("Got response from Open AI", slog.String("response", response))

	suggestions := strings.Split(response, "\n")
	for _, suggestion := range suggestions {
		entries, err := b.Suggest(ctx, state.Entity{Role: state.RoleSystem}, suggestion)
		if err != nil {
			return err
		}

		// For now, just use the first theme that returns results as the suggestion
		// mechanism is shared with the users. Don't lock them out.
		if len(entries) > 0 {
			return b.Extrapolate(ctx)
		}
	}

	return nil
}

func (b *Bot) extrapolateWithHistory(ctx context.Context) error {
	var lookback strings.Builder
	entries := b.state.History.PeakBackN(b.state.Config.ExtrapolationLookback)
	for i, entry := range entries {
		fmt.Fprintf(&lookback, "%d. %s\n", i+1, entry.Title)
	}
	// TODO: It's ugly to unlock here when it was locked elsewhere (Extrapolate)
	b.mutex.Unlock()

	slog.Debug("Extrapolating songs based on history", slog.String("history", lookback.String()))
	res, err := b.llm.Chat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: b.state.Config.Prompt,
			},
			{
				Role:    llm.RoleAssistant,
				Content: lookback.String(),
			},
			{
				Role:    llm.RoleUser,
				Content: "provide similar songs",
			},
		},
	})
	if err != nil {
		return err
	}

	if len(res.Message.Content) == 0 {
		slog.Debug("No response from LLM")
		return nil
	}

	if res.Message.Content == "no results" {
		slog.Debug("No results from LLM")
		return nil
	}
	slog.Debug("Got response from Open AI", slog.String("response", res.Message.Content))

	// TODO: Assume default prompt for now
	lines := strings.Split(res.Message.Content, "\n")
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
	if b.isStreaming {
		return fmt.Errorf("already playing")
	}

	b.shouldPlay = true

	failures := 0

	for failures < 5 && b.shouldPlay {
		b.mutex.Lock()
		entry, ok := b.state.Queue.Pop()
		b.mutex.Unlock()

		if !ok {
			if b.LLMEnabled() && b.state.Config.ExtrapolateWhenEmpty {
				slog.Debug("Playlist is empty, extrapolating")
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
		} else if errors.Is(err, ErrUnsupportedAudioCodec) {
			slog.Error("Failed to play unsupported entry", slog.String("title", entry.Title), slog.Any("error", err))
			// Skip to next
			// TODO: Communicate the failure - write in chat (currently no way to send
			// messages from the bot) or get the URL immediately on queue and complain
			// then?
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

	ctx, cancel := context.WithCancel(context.Background())

	b.mutex.Lock()
	b.currentEntry = &entry
	b.isStreaming = true
	b.cancelStream = cancel
	b.state.History.AddEntry(entry)
	b.mutex.Unlock()

	reader, writer := io.Pipe()
	webmReader := webm.NewReader(reader)
	go func() {
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
				cancel()
				break
			}

			opus <- frame.Payload
		}
	}()

	b.state.Metrics.SongsPlayed.Inc()
	b.state.Metrics.ActiveStreams.Inc()

	playbackStarted := time.Now()

	// TODO: Catch specific errors, like unsupported codec / not found
	// TODO: err is "signal: killed" on normal shut down (e.g. skip), should be
	// ignored
	err := ytdlp.Stream(ctx, entry.URI, writer)
	var ytdlpErr ytdlp.Error
	if errors.As(err, &ytdlpErr) {
		slog.Error("Failed to stream using yt-dlp", slog.String("stderr", ytdlpErr.Stderr))
		return err
	} else if err != nil {
		return err
	}

	b.state.Metrics.DurationPlayed.Add(time.Since(playbackStarted).Seconds())
	b.state.Metrics.ActiveStreams.Dec()

	b.mutex.Lock()
	b.currentEntry = nil
	b.isStreaming = false
	b.cancelStream = nil
	b.mutex.Unlock()

	slog.Debug("Stopped playing stream", slog.String("source", string(entry.Source)), slog.String("uri", entry.URI), slog.String("title", entry.Title))
	return err
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
	if b.isStreaming {
		b.cancelStream()
	}

	b.ExtrapolationType = ExtrapolationTypeNone
	if b.state.Config.ExtrapolateWhenEmpty {
		b.ExtrapolationType = ExtrapolationTypeHistory
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

	if b.isStreaming {
		b.state.Queue.PopN(n - 1)
		b.cancelStream()
	}
}

// NowPlaying returns the current playlist entry, or nil if nothing is playing.
func (b *Bot) NowPlaying() *state.PlaylistEntry {
	return b.currentEntry
}

func (b *Bot) LLMEnabled() bool {
	return b.llm != nil
}
