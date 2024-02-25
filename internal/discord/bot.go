package discord

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/bwmarrin/discordgo"
)

type Conn struct {
	state   *state.State
	bot     *bot.Bot
	discord *discordgo.Session

	isConnected bool
}

func Dial(state *state.State, bot *bot.Bot) (*Conn, error) {
	conn := &Conn{
		state: state,
		bot:   bot,
	}

	var err error
	conn.discord, err = discordgo.New("Bot " + state.Config.DiscordBotToken)
	if err != nil {
		return nil, err
	}

	conn.discord.Identify.Intents = discordgo.IntentGuilds | discordgo.IntentGuildVoiceStates

	if err := conn.discord.Open(); err != nil {
		return nil, err
	}

	commands := map[string]*Command{
		"play": {
			Handler: conn.handlePlayCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Start playing music in the voice channel you're in",
			},
		},
		"queue": {
			Handler: conn.handleQueueCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Queue music in the voice channel you're in",
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
		"playlist": {
			Handler: conn.handlePlaylistCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Print playlist",
			},
		},
		"recent": {
			Handler: conn.handlePlaylistCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Print recent history",
			},
		},
		"stop": {
			Handler: conn.handleStopCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Stop any music currently playing",
			},
		},
		"skip": {
			Handler: conn.handleSkipCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Skip the current song",
			},
		},
	}

	if bot.OpenAIEnabled() {
		commands["suggest"] = &Command{
			Handler: conn.handleSuggestCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Suggest songs, artists or vibes to be played using AI",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "query",
						Type:        discordgo.ApplicationCommandOptionString,
						Description: "LLM query",
						Required:    true,
					},
				},
			},
		}

		commands["suggestions"] = &Command{
			Handler: conn.handlePlaylistCommand,
			ApplicationCommand: &discordgo.ApplicationCommand{
				Description: "Print suggestions",
			},
		}
	}

	slog.Debug("Registering commands")
	// TODO: Do we need to remove the commands when we leave?
	for commandName, command := range commands {
		command.ApplicationCommand.Name = commandName
		_, err := conn.discord.ApplicationCommandCreate(conn.discord.State.User.ID, "", command.ApplicationCommand)
		if err != nil {
			conn.discord.Close()
			return nil, err
		}
	}

	// Add a handler for command interactions
	conn.discord.AddHandler(func(session *discordgo.Session, event *discordgo.InteractionCreate) {
		commandName := event.ApplicationCommandData().Name
		slog.Debug("Got command request", slog.String("name", commandName))

		command, ok := commands[commandName]
		if !ok {
			slog.Warn("Got command interaction for unknown command", slog.String("name", commandName))
			return
		}

		if err := session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Error("Failed to acknowledge command", slog.Any("error", err))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := command.Handler(ctx, session, event); err != nil {
			slog.Error("Failed to handle command", slog.Any("error", err))
			conn.updateResponse(session, event, "An error occured. Try again in a little while.")
		}
	})

	slog.Info("Bot started")
	return conn, nil
}

func (c *Conn) Ready() bool {
	return c.discord.DataReady
}

func (c *Conn) Close() error {
	return c.discord.Close()
}

func (c *Conn) updateResponse(session *discordgo.Session, event *discordgo.InteractionCreate, content string) error {
	_, err := session.FollowupMessageCreate(event.Interaction, false, &discordgo.WebhookParams{
		Content: content,
	})
	return err
}

func (c *Conn) parseVoiceChannel(session *discordgo.Session, event *discordgo.InteractionCreate) (string, string, error) {
	// Find the channel that the message came from.
	channel, err := c.discord.State.Channel(event.ChannelID)
	if err != nil {
		return "", "", fmt.Errorf("failed to identify event's channel")
	}

	// Find the guild for that channel.
	guild, err := c.discord.State.Guild(channel.GuildID)
	if err != nil {
		return "", "", fmt.Errorf("failed to identify event's guild")
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
		return "", "", c.updateResponse(session, event, "You need to be in a voice channel to do that.")
	}

	return guild.ID, voiceChannelID, nil
}

func (c *Conn) parseQuery(session *discordgo.Session, event *discordgo.InteractionCreate) (string, error) {
	options := event.ApplicationCommandData().Options

	query := ""
	for _, option := range options {
		if option.Name == "query" {
			query = option.StringValue()
		}
	}

	if query == "" {
		return "", c.updateResponse(session, event, "You need to specify a search query.")
	}

	return query, nil
}

func (c *Conn) handlePlayCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	guildID, voiceChannelID, err := c.parseVoiceChannel(session, event)
	if err != nil {
		return err
	}

	c.connectBot(guildID, voiceChannelID)

	return c.updateResponse(session, event, "On my way!")
}

func (c *Conn) handleQueueCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	guildID, voiceChannelID, err := c.parseVoiceChannel(session, event)
	if err != nil {
		return err
	}

	query, err := c.parseQuery(session, event)
	if err != nil {
		return err
	}

	name := event.Member.Nick
	if name == "" {
		name = event.Member.User.Username
	}
	entity := state.Entity{
		Role: state.RoleUser,
		ID:   fmt.Sprintf("%s/%s", guildID, event.Member.User.ID),
		Name: name,
	}
	results, err := c.bot.Queue(ctx, query, entity, nil)
	if err != nil {
		slog.Error("Failed to queue query results", slog.Any("error", err))
		return c.updateResponse(session, event, "I can't do that right now. Try again in a little while.")
	}

	if len(results) == 0 {
		return c.updateResponse(session, event, "I couldn't find anything for you.")
	}

	c.connectBot(guildID, voiceChannelID)

	return c.updateResponse(session, event, fmt.Sprintf("Queued %d songs", len(results)))
}

func (c *Conn) handleSuggestCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	guildID, voiceChannelID, err := c.parseVoiceChannel(session, event)
	if err != nil {
		return err
	}

	query, err := c.parseQuery(session, event)
	if err != nil {
		return err
	}

	name := event.Member.Nick
	if name == "" {
		name = event.Member.User.Username
	}
	entity := state.Entity{
		Role: state.RoleUser,
		ID:   fmt.Sprintf("%s/%s", guildID, event.Member.User.ID),
		Name: name,
	}
	results, err := c.bot.Suggest(ctx, entity, query)
	if err != nil {
		slog.Error("Failed to suggest songs", slog.Any("error", err))
		return c.updateResponse(session, event, "I can't do that right now. Try again in a little while.")
	}

	if len(results) == 0 {
		return c.updateResponse(session, event, "I couldn't find anything for you.")
	}

	c.connectBot(guildID, voiceChannelID)

	return c.updateResponse(session, event, "I'll take your suggestions into account.")
}

func (c *Conn) handleSkipCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	c.bot.Skip()
	return c.updateResponse(session, event, "Skipping song.")
}

func (c *Conn) handleStopCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	c.bot.Stop()
	return c.updateResponse(session, event, "Stopping.")
}

func (c *Conn) handlePlaylistCommand(ctx context.Context, session *discordgo.Session, event *discordgo.InteractionCreate) error {
	var playlist *state.Playlist
	reverse := false
	format := ""
	if event.ApplicationCommandData().Name == "playlist" {
		playlist = c.state.Queue
		format = "{{.Index}}. {{.EntityName}} queued {{.RelativeTime}} - **{{.Title}}**\n"
	} else if event.ApplicationCommandData().Name == "suggestions" {
		playlist = c.state.Suggestions
		format = "{{.Index}}. **{{.Title}}**\n"
	} else if event.ApplicationCommandData().Name == "recent" {
		playlist = c.state.History
		reverse = true
		format = "{{.Index}}. {{.EntityName}} played {{.RelativeTime}} - **{{.Title}}**\n"
	} else {
		panic("discord: unexpected command")
	}

	contents, err := playlist.Format(format, 20, reverse)
	if err != nil {
		return err
	}
	if contents == "" {
		contents = "No songs"
	}

	return c.updateResponse(session, event, contents)
}

func (c *Conn) connectBot(guildID string, voiceChannelID string) {
	go func() {
		if c.isConnected {
			return
		}

		slog.Debug("Connecting bot to voice channel", slog.String("guildId", guildID), slog.String("voiceChannelID", voiceChannelID))
		channel, err := c.discord.ChannelVoiceJoin(guildID, voiceChannelID, false, true)
		if err != nil {
			slog.Error("Failed to join channel", slog.Any("error", err))
			return
		}
		defer func() {
			channel.Speaking(false)
			channel.Disconnect()
			c.isConnected = false
			c.discord.UpdateStatusComplex(discordgo.UpdateStatusData{
				Activities: []*discordgo.Activity{},
			})
		}()
		c.isConnected = true

		if c.state.Config.LogLevel == slog.LevelDebug {
			channel.LogLevel = discordgo.LogDebug
		}

		time.Sleep(250 * time.Millisecond)

		channel.Speaking(true)

		// Continuously update the bot's presence to reflect the currently playing
		// song
		songs := make(chan string)
		go func() {
			for song := range songs {
				err := c.discord.UpdateStatusComplex(discordgo.UpdateStatusData{
					Activities: []*discordgo.Activity{
						{
							Name: song,
							Type: discordgo.ActivityTypeListening,
						},
					},
				})
				if err != nil {
					slog.Error("Failed to set presence", slog.Any("error", err))
				}
			}
		}()

		slog.Debug("Bot is connected to voice channel, starting to play")
		if err = c.bot.Play(channel.OpusSend, songs); err != nil {
			slog.Error("Failed to play", slog.Any("error", err))
		}

		// Make sure buffers are emptied
		time.Sleep(500 * time.Millisecond)
	}()
}
