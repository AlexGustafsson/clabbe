package youtube

import (
	"context"
	"io"
	"log/slog"
	"sort"

	"github.com/AlexGustafsson/clabbe/internal/streaming"
	"github.com/kkdai/youtube/v2"
)

var _ (streaming.AudioStream) = (*AudioStream)(nil)

type AudioStream struct {
	title  string
	reader io.ReadCloser
	size   int64
}

// Close implements streaming.AudioStream.
func (s *AudioStream) Close() error {
	return s.reader.Close()
}

// Read implements streaming.AudioStream.
func (s *AudioStream) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

// Size implements streaming.AudioStream.
func (s *AudioStream) Size() int64 {
	return s.size
}

// Title implements streaming.AudioStream.
func (s *AudioStream) Title() string {
	return s.title
}

type StreamOptions struct {
	Client *youtube.Client
	Sort   func(i int, j int, formats []youtube.Format) bool
}

func DefaultAudioSort(i int, j int, formats []youtube.Format) bool {
	left := formats[i]
	right := formats[j]

	// Prioritize opus (codec efficiency)
	leftIsOpus := left.MimeType == `audio/webm; codecs="opus"`
	rightIsOpus := right.MimeType == `audio/webm; codecs="opus"`
	if leftIsOpus && !rightIsOpus {
		return true
	}

	// Prioritize high bitrate (quality)
	return left.Bitrate > right.Bitrate
}

func NewAudioStream(ctx context.Context, id string, options *StreamOptions) (*AudioStream, error) {
	if options == nil {
		options = &StreamOptions{}
	}

	client := options.Client
	if client == nil {
		client = &youtube.Client{}
	}

	sortFunc := options.Sort
	if sortFunc == nil {
		sortFunc = DefaultAudioSort
	}

	slog.Debug("Fetching video metadata")
	video, err := client.GetVideoContext(ctx, id)
	if err != nil {
		return nil, err
	}
	slog.Debug("Fetched video", slog.String("title", video.Title), slog.String("id", video.ID))

	formats := []youtube.Format(video.Formats.WithAudioChannels())
	sort.SliceStable(formats, func(i, j int) bool {
		return sortFunc(i, j, formats)
	})

	slog.Debug("Retrieving audio stream")
	reader, size, err := client.GetStreamContext(ctx, video, &formats[0])
	if err != nil {
		return nil, err
	}

	// Close the reader when the context is canceled
	go func() {
		<-ctx.Done()
		slog.Debug("Context canceled, closing stream reader")
		reader.Close()
	}()

	return &AudioStream{
		title:  video.Title,
		reader: reader,
		size:   size,
	}, nil
}
