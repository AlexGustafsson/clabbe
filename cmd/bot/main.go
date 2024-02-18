package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/discord"
	"github.com/AlexGustafsson/clabbe/internal/openai"
)

func run(ctx context.Context, token string, openAIKey string) error {

	var openAIClient *openai.Client
	if openAIKey != "" {
		openAIClient = openai.NewClient(openAIKey)
	}

	bot := bot.New(openAIClient)

	conn, err := discord.Dial(bot, token)
	if err != nil {
		slog.Error("Failed to start bot", slog.Any("error", err))
		return err
	}

	<-ctx.Done()
	bot.Stop()
	conn.Close()

	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	discordBotToken, ok := os.LookupEnv("DISCORD_BOT_TOKEN")
	if !ok {
		slog.Error("Missing required environment variable DISCORD_BOT_TOKEN")
		os.Exit(1)
	}

	openAIKey, ok := os.LookupEnv("OPENAI_API_KEY")
	if !ok {
		slog.Warn("Missing OpenAI API key - disabling advanced features")
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

	if err := run(ctx, discordBotToken, openAIKey); err != nil {
		slog.Error("Program was unsuccessful", slog.Any("error", err))
		os.Exit(1)
	}
}
