package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlexGustafsson/clabbe/internal/ffmpeg"
	"github.com/AlexGustafsson/clabbe/internal/youtube"
	"github.com/AlexGustafsson/clabbe/internal/ytdlp"
)

func run(ctx context.Context, query string) error {
	results, err := youtube.Search(ctx, query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no results")
	}

	slog.Debug("Initializing new player")
	player, err := ffmpeg.NewPlayer()
	if err != nil {
		slog.Error("Failed to initialize player", slog.Any("error", err))
		return err
	}

	slog.Debug("Starting stream")
	err = ytdlp.Stream(ctx, results[0].ID, player)
	if err != nil {
		slog.Error("Failed to stream", slog.Any("error", err))
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

	// Exit on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		abort := make(chan os.Signal, 1)
		signal.Notify(abort, syscall.SIGINT, syscall.SIGTERM)
		caught := 0
		for {
			<-abort
			caught++
			if caught == 1 {
				slog.Info("Caught signal, exiting gracefully")
				cancel()
			} else {
				slog.Info("Caught signal, exiting now")
				os.Exit(1)
			}
		}
	}()

	if err := run(ctx, os.Args[1]); err != nil {
		slog.Error("Program was unsuccessful", slog.Any("error", err))
		os.Exit(1)
	}
}
