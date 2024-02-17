package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/AlexGustafsson/clabbe/internal/ffmpeg"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
)

func run(ctx context.Context, query string) error {
	results, err := youtube.Search(ctx, query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no results")
	}

	slog.Debug("Creating audio stream", slog.String("id", results[0]))
	stream, err := youtube.NewAudioStream(ctx, results[0], nil)
	if err != nil {
		slog.Error("Failed to fetch audio stream", slog.Any("error", err))
		return err
	}

	defer stream.Close()

	normalizedStream, err := ffmpeg.NewNormalizedAudioStream(stream)
	if err != nil {
		slog.Error("Failed to normalize stream", slog.Any("error", err))
		return err
	}

	if err := ffmpeg.Play(normalizedStream); err != nil {
		slog.Error("Failed to play audio stream", slog.Any("error", err))
		return err
	}

	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <query>\n", os.Args[0])
		os.Exit(1)
	}

	if err := run(context.Background(), os.Args[1]); err != nil {
		os.Exit(1)
	}
}
