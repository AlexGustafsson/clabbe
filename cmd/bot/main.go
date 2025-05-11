package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/discord"
	"github.com/AlexGustafsson/clabbe/internal/llm"
	"github.com/AlexGustafsson/clabbe/internal/llm/ollama"
	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func run(ctx context.Context, state *state.State) error {
	var llmClient llm.Client
	if state.Config.Ollama != nil {
		url, err := url.Parse(state.Config.Ollama.Endpoint)
		if err != nil {
			slog.Error("Failed to parse ollama URL", slog.Any("error", err))
			return err
		}

		llmClient = ollama.NewClient(url, state.Config.Ollama.Model, nil)
	}

	bot := bot.New(state, llmClient)
	var conn *discord.Conn

	if state.Config.Prometheus.Enabled {
		if err := prometheus.DefaultRegisterer.Register(state.Metrics); err != nil {
			return err
		}

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", state.Config.Prometheus.Port))
		if err != nil {
			return err
		}

		mux := http.NewServeMux()

		mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
			// Default 200 OK
		})

		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if conn == nil || !conn.Ready() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

			// Default 200 OK
		})

		mux.Handle("/metrics", promhttp.Handler())

		server := http.Server{
			Handler: mux,
		}

		// Serve the API
		go func() {
			slog.Info("Serving metrics", slog.String("address", listener.Addr().String()))
			err := server.Serve(listener)
			if err != nil && err != http.ErrServerClosed {
				slog.Error("Failed to run metrics HTTP server")
			}
		}()

		// Close the server gracefully on shutdown
		go func() {
			<-ctx.Done()
			server.Close()
		}()
	}

	// Connect to Discord
	slog.Info("Connecting bot to Discord")
	var err error
	conn, err = discord.Dial(state, bot)
	if err != nil {
		slog.Error("Failed to start bot", slog.Any("error", err))
		return err
	}

	// Wait for the program to be stopped before gracefully exiting
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

	if state.Config.Ollama == nil {
		slog.Warn("Missing ollama config - disabling advanced features")
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
