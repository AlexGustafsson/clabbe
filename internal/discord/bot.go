package discord

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/ffmpeg"
	"github.com/AlexGustafsson/clabbe/internal/streaming"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
	"github.com/bwmarrin/discordgo"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

type Bot struct {
	discord *discordgo.Session

	mutex         sync.Mutex
	currentStream streaming.AudioStream
}

func NewBot(token string) (*Bot, error) {
	bot := &Bot{}

	var err error
	bot.discord, err = discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	bot.discord.Identify.Intents = discordgo.IntentGuilds | discordgo.IntentGuildVoiceStates

	slog.Debug("Connecting bot")
	if err := bot.discord.Open(); err != nil {
		return nil, err
	}

	commands := map[string]*Command{
		"play": {
			Handler: bot.handlePlayCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Play music in the voice channel you're in",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "query",
						Type:        discordgo.ApplicationCommandOptionString,
						Description: "YouTube search query",
						Required:    true,
					},
				},
			},
		},
		"stop": {
			Handler: bot.handleStopCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Stop any music currently playing",
			},
		},
	}

	slog.Debug("Registering bot commands")
	// TODO: Do we need to remove the commands when we leave?
	for commandName, command := range commands {
		command.ApplicationCommand.Name = commandName
		_, err := bot.discord.ApplicationCommandCreate(bot.discord.State.User.ID, "", command.ApplicationCommand)
		if err != nil {
			bot.discord.Close()
			return nil, err
		}
	}

	// Add a handler for command interactions
	bot.discord.AddHandler(func(session *discordgo.Session, event *discordgo.InteractionCreate) {
		commandName := event.ApplicationCommandData().Name
		command, ok := commands[commandName]
		if !ok {
			slog.Warn("Got command interaction for unknown command", slog.String("name", commandName))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := command.Handler(ctx, session, event); err != nil {
			slog.Error("Failed to handle command", slog.Any("error", err))
		}
	})

	slog.Info("Bot started")

	return bot, nil
}

func (b *Bot) Stop() error {
	return b.discord.Close()
}

func (b *Bot) handlePlayCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	options := event.ApplicationCommandData().Options

	query := ""
	for _, option := range options {
		if option.Name == "query" {
			query = option.StringValue()
		}
	}

	if query == "" {
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need to specify a search query.",
			},
		})
	}

	// Find the channel that the message came from.
	channel, err := b.discord.State.Channel(event.ChannelID)
	if err != nil {
		return fmt.Errorf("failed to identify event's channel")
	}

	// Find the guild for that channel.
	guild, err := b.discord.State.Guild(channel.GuildID)
	if err != nil {
		return fmt.Errorf("failed to identify event's guild")
	}

	// Find the voice channel of the sender
	voiceChannelID := ""
	for _, voiceStates := range guild.VoiceStates {
		if voiceStates.UserID == event.Member.User.ID {
			voiceChannelID = voiceStates.ChannelID
			break
		}
	}

	if voiceChannelID == "" {
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need to be in a voice channel to play music.",
			},
		})
	}

	// TODO: Playlist
	if b.currentStream != nil {
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "A song is already playing. Try again later.",
			},
		})
	}

	results, err := youtube.Search(ctx, query)
	if err != nil {
		slog.Error("Failed to perform search", slog.Any("error", err))
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sorry, I can't search for that right now. Try again in a little while.",
			},
		})
	}

	if len(results) == 0 {
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I couldn't find anything for the provided query. Try another.",
			},
		})
	}

	stream, err := youtube.NewAudioStream(context.Background(), results[0], nil)
	if err != nil {
		slog.Error("Failed to create YouTube audio stream", slog.Any("error", err))
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sorry, I can't play that right now. Try again in a little while.",
			},
		})
	}

	normalizedStream, err := ffmpeg.NewNormalizedAudioStream(stream)
	if err != nil {
		slog.Error("Failed to create normalized stream", slog.Any("error", err))
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sorry, I can't play that right now. Try again in a little while.",
			},
		})
	}

	b.currentStream = stream

	go func() {
		defer func() {
			stream.Close()
			b.currentStream = nil
		}()

		channel, err := b.discord.ChannelVoiceJoin(guild.ID, voiceChannelID, false, true)
		if err != nil {
			slog.Error("Failed to join channel", slog.Any("error", err))
			return
		}
		defer func() {
			channel.Speaking(false)
			channel.Disconnect()
		}()

		channel.LogLevel = discordgo.LogDebug

		time.Sleep(250 * time.Millisecond)

		channel.Speaking(true)

		reader, _, err := oggreader.NewWith(normalizedStream)
		if err != nil {
			slog.Error("Failed to create ogg reader", slog.Any("error", err))
			return
		}

		for {
			page, _, err := reader.ParseNextPage()
			if err != nil {
				if err == io.EOF {
					slog.Debug("Stream ended")
				} else {
					slog.Error("Failed to read ogg page", slog.Any("error", err))
				}
				break
			}

			channel.OpusSend <- page
		}

		// Make sure buffers are emptied
		time.Sleep(500 * time.Millisecond)
	}()

	return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Playing: %s", stream.Title()),
		},
	})
}

func (b *Bot) handleStopCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.currentStream != nil {
		title := b.currentStream.Title()
		b.currentStream.Close()
		return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Stopped %s", title),
			},
		})
	}

	return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "No music is playing",
		},
	})
}
