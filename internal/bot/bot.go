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
	"github.com/AlexGustafsson/clabbe/internal/streaming"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
	"github.com/AlexGustafsson/clabbe/internal/webm"
)

var (
	ErrNoStreamPlaying = errors.New("no stream is playing")
)

type Bot struct {
	mutex       sync.Mutex
	playlist    *Playlist
	suggestions *Playlist
	history     *Playlist

	shouldPlay    bool
	currentStream streaming.AudioStream

	openai *openai.Client

	InterpolateWhenEmpty bool
}

func New(openai *openai.Client) *Bot {
	return &Bot{
		playlist:    NewPlaylist(),
		suggestions: NewPlaylist(),
		history:     NewPlaylist(),
		openai:      openai,

		InterpolateWhenEmpty: true,
	}
}

// Search performs a search for content.
// Returns the first result. When using AI, returns the first result for each
// additional query provided by the AI.
func (b *Bot) Search(ctx context.Context, query string, useAI bool) ([]youtube.SearchResult, error) {
	queries := make([]string, 0)

	if useAI && b.openai != nil {
		slog.Debug("Extrapolating search using AI", slog.String("query", query))
		res, err := b.openai.FetchCompletion(ctx, &openai.CompletionRequest{
			Messages: []openai.Message{
				{
					Role:    openai.RoleSystem,
					Content: DefaultPrompt,
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

		if len(res.Choices) > 0 {
			response := res.Choices[0].Message.Content
			slog.Debug("Got response from Open AI", slog.String("response", response))

			// TODO: Assume default prompt for now
			entries := strings.Split(response, "\n")
			for _, entry := range entries {
				// The default output has indexes, remove them
				_, query, _ := strings.Cut(entry, " ")
				queries = append(queries, query)
			}
		}
	} else {
		queries = append(queries, query)
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
	// Source defaults to "human".
	Source string
}

// Queue performs a search for content and adds the top result to the playlist.
func (b *Bot) Queue(ctx context.Context, query string, options *QueueOptions) ([]youtube.SearchResult, error) {
	slog.Debug("Got request to queue songs", slog.String("query", query))
	if options == nil {
		options = &QueueOptions{}
	}

	useAI := options.UseAI
	source := options.Source
	if source == "" {
		source = "human"
	}

	results, err := b.Search(ctx, query, useAI)
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		slog.Debug("Got results", slog.Any("results", results))
		b.mutex.Lock()
		for _, result := range results {
			b.playlist.Push(PlaylistEntry{
				Source: source,
				Added:  time.Now(),
				ID:     result.ID,
				Title:  result.Title,
			})
		}
		b.mutex.Unlock()
	} else {
		slog.Debug("No results")
	}

	return results, nil
}

type SuggestOptions struct {
	// UseAI defaults to false.
	UseAI bool
}

// Suggest adds the results as a basis for songs to play when interpolating.
func (b *Bot) Suggest(ctx context.Context, query string) error {
	slog.Debug("Got suggestion", slog.String("query", query))
	results, err := b.Search(ctx, query, true)
	if err != nil {
		return err
	}

	slog.Debug("Got results", slog.Any("results", results))
	b.mutex.Lock()
	for _, result := range results {
		b.suggestions.Push(PlaylistEntry{
			Source: "human",
			Added:  time.Now(),
			ID:     result.ID,
			Title:  result.Title,
		})
	}
	b.mutex.Unlock()

	return nil
}

// Interpolate adds some entries to the playlist based on suggestions and
// history.
func (b *Bot) Interpolate(ctx context.Context) error {
	// If possible, use the suggestions immediately
	b.mutex.Lock()
	suggestions := b.suggestions.PopN(5)
	if len(suggestions) > 0 {
		slog.Debug("There were unused suggestions, use them first")
		for _, suggestion := range suggestions {
			b.playlist.Push(suggestion)
		}
		b.mutex.Unlock()
		return nil
	}

	// If no suggestions were ready, interpolate based on history
	if b.openai == nil {
		b.mutex.Unlock()
		return fmt.Errorf("missing required Open AI client")
	}

	var lookback strings.Builder
	entries := b.history.PeakN(10)
	for i, entry := range entries {
		fmt.Fprintf(&lookback, "%d: %s", i+1, entry.Title)
	}
	b.mutex.Unlock()

	slog.Debug("Completing based off of history", slog.String("history", lookback.String()))
	res, err := b.openai.FetchCompletion(ctx, &openai.CompletionRequest{
		Messages: []openai.Message{
			{
				Role:    openai.RoleSystem,
				Content: DefaultPrompt,
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

	if len(res.Choices) == 0 {
		slog.Debug("No response from LLM")
		return nil
	}

	response := res.Choices[0].Message.Content
	slog.Debug("Got response from Open AI", slog.String("response", response))

	if response == "no results" {
		slog.Debug("No results from LLM")
		return nil
	}

	// TODO: Assume default prompt for now
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		_, query, _ := strings.Cut(line, " ")
		if _, err := b.Queue(ctx, strings.TrimSpace(query), &QueueOptions{Source: "ai"}); err != nil {
			return err
		}
	}

	return nil
}

// Play starts playing content, sending windows of OPUS-encoded audio to the
// provided channel.
func (b *Bot) Play(opus chan<- []byte) error {
	if b.currentStream != nil {
		return fmt.Errorf("already playing")
	}

	b.shouldPlay = true

	for {
		b.mutex.Lock()
		entry, ok := b.playlist.Pop()
		b.mutex.Unlock()

		if !ok {
			if b.InterpolateWhenEmpty {
				slog.Debug("Playlist is empty, interpolating")
				err := b.Interpolate(context.Background())
				if err != nil {
					return err
				}
				continue
			} else {
				slog.Debug("Playlist is empty, closing")
				return nil
			}
		}

		if err := b.playOnce(entry, opus); err != nil {
			return err
		}
	}
}

// playOnce plays the entry, sending windows of OPUS-encoded audio to the
// provided channel.
func (b *Bot) playOnce(entry PlaylistEntry, opus chan<- []byte) error {
	slog.Debug("Playing", slog.String("id", entry.ID), slog.String("Title", entry.Title))
	defer func() {
		b.currentStream = nil
	}()

	b.mutex.Lock()
	stream, err := youtube.NewAudioStream(context.Background(), entry.ID, nil)
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

	b.currentStream = stream
	b.history.PushFront(entry)
	b.mutex.Unlock()

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

	slog.Debug("Stopped playing stream", slog.String("id", entry.ID), slog.String("title", stream.Title()))
	return nil
}

// ClearPlaylist clears all entries of the playlist.
func (b *Bot) ClearPlaylist() {
	slog.Debug("Clearing playlist")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.playlist.Clear()
}

func (b *Bot) ClearSuggestions() {
	slog.Debug("Clearing suggestions")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.suggestions.Clear()
}

// Stop stops the currently playing stream and clears the playlist.
func (b *Bot) Stop() {
	slog.Debug("Stopping playing stream and clearing playlist")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.shouldPlay = false
	b.playlist.Clear()
	b.suggestions.Clear()
	if b.currentStream != nil {
		b.currentStream.Close()
	}
}

// Skip stops the currently playing stream.
func (b *Bot) Skip() {
	slog.Debug("Skipping playing stream")
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.currentStream != nil {
		b.currentStream.Close()
	}
}

func (b *Bot) IsPlaying() bool {
	return b.currentStream != nil
}

func (b *Bot) OpenAIEnabled() bool {
	return b.openai != nil
}

func (b *Bot) Playlist() *Playlist {
	return b.playlist
}

func (b *Bot) Suggestions() *Playlist {
	return b.suggestions
}
