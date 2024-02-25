package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlexGustafsson/clabbe/internal/ffmpeg"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
	"github.com/AlexGustafsson/clabbe/internal/webm"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

func run(ctx context.Context, query string) error {
	results, err := youtube.Search(ctx, query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no results")
	}

	slog.Debug("Creating audio stream", slog.String("id", results[0].ID))
	stream, err := youtube.NewAudioStream(ctx, results[0].ID, nil)
	if err != nil {
		slog.Error("Failed to fetch audio stream", slog.Any("error", err))
		return err
	}

	go func() {
		<-ctx.Done()
		slog.Debug("Closing stream")
		stream.Close()
	}()

	slog.Debug("Initializing new player")
	player, err := ffmpeg.NewPlayer()
	if err != nil {
		slog.Error("Failed to initialize player", slog.Any("error", err))
		return err
	}

	// As plain OPUS has very little support in players, mux it back to an ogg
	// file
	ogg, err := oggwriter.NewWith(player, 44000, 2)
	if err != nil {
		slog.Error("Failed to create OGG writer", slog.Any("error", err))
		return err
	}
	defer ogg.Close()

	reader := webm.NewReader(stream)
	for {
		frame, err := reader.Read()
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

		ogg.WriteRTP(&rtp.Packet{
			Header: rtp.Header{
				Timestamp: uint32(frame.Timecode),
			},
			Payload: frame.Payload,
		})
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
