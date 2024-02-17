package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlexGustafsson/clabbe/internal/discord"
)

func run(ctx context.Context, token string) error {

	bot, err := discord.NewBot(token)
	if err != nil {
		slog.Error("Failed to start bot", slog.Any("error", err))
		return err
	}

	<-ctx.Done()
	bot.Stop()

	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	token, ok := os.LookupEnv("DISCORD_BOT_TOKEN")
	if !ok {
		slog.Error("Missing required environment variable DISCORD_BOT_TOKEN")
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

	if err := run(ctx, token); err != nil {
		slog.Error("Program was unsuccessful", slog.Any("error", err))
		os.Exit(1)
	}
}
