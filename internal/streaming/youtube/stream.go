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
	title    string
	reader   io.ReadCloser
	size     int64
	mimeType string
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

// MimeType implements streaming.AudioStream.
func (s *AudioStream) MimeType() string {
	return s.mimeType
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

	format := formats[0]

	slog.Debug("Retrieving audio stream")
	reader, size, err := client.GetStreamContext(ctx, video, &format)
	if err != nil {
		return nil, err
	}

	return &AudioStream{
		title:    video.Title,
		reader:   reader,
		size:     size,
		mimeType: format.MimeType,
	}, nil
}
