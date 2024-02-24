package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/discord"
	"github.com/AlexGustafsson/clabbe/internal/openai"
	"github.com/AlexGustafsson/clabbe/internal/state"
)

func run(ctx context.Context, state *state.State) error {
	var openAIClient *openai.Client
	if state.Config.OpenAIKey != "" {
		openAIClient = openai.NewClient(state.Config.OpenAIKey)
	}

	bot := bot.New(state, openAIClient)

	conn, err := discord.Dial(state, bot)
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
		Level: slog.LevelInfo,
	})))

	config := flag.String("config", "", "path to config directory")
	flag.Parse()

	if *config == "" {
		slog.Error("Missing required flag config")
		os.Exit(1)
	}

	state, err := state.LoadOrInit(*config)
	if err != nil {
		slog.Error("Failed to load state", slog.Any("error", err))
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: state.Config.LogLevel,
	})))

	if state.Config.DiscordBotToken == "" {
		slog.Error("Missing required config for Discord bot token")
		os.Exit(1)
	}

	if state.Config.OpenAIKey == "" {
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

	// Continously persist the state
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Debug("Storing the state")
				if err := state.Store(); err != nil {
					slog.Error("Failed to store state", slog.Any("error", err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err = run(ctx, state)
	slog.Debug("Storing state before exiting")
	if err := state.Store(); err != nil {
		slog.Error("Failed to store state on exit", slog.Any("error", err))
	}
	if err != nil {
		slog.Error("Program was unsuccessful", slog.Any("error", err))
		os.Exit(1)
	}
}
