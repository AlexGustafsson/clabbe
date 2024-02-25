package discord

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/bwmarrin/discordgo"
)

// Conn is a Discord bot connection.
type Conn struct {
	state   *state.State
	bot     *bot.Bot
	discord *discordgo.Session

	isConnected bool

	commands map[string]Command
}

// Dial connects to Discord. Returns the open connection or an error if
// connecting fails.
func Dial(state *state.State, bot *bot.Bot) (*Conn, error) {
	conn := &Conn{
		state: state,
		bot:   bot,

		commands: make(map[string]Command),
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

	slog.Debug("Registering commands")

	for _, command := range commands {
		options := make([]*discordgo.ApplicationCommandOption, len(command.Options))
		for i, o := range command.Options {
			options[i] = &discordgo.ApplicationCommandOption{
				Name:        o.Name,
				Description: o.Description,
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    o.Required,
			}
		}

		_, err := conn.discord.ApplicationCommandCreate(conn.discord.State.User.ID, "", &discordgo.ApplicationCommand{
			Name:        command.Name,
			Description: command.Description,
			Options:     options,
		})
		if err != nil {
			conn.discord.Close()
			return nil, err
		}
		conn.commands[command.Name] = command
	}

	slog.Debug("Trying to clean up old commands, if any")
	commands, err := conn.discord.ApplicationCommands(conn.discord.State.Application.ID, "")
	if err == nil {
		for _, command := range commands {
			if _, ok := conn.commands[command.Name]; !ok {
				slog.Debug("Removing command", slog.String("name", command.Name), slog.String("id", command.ID))
				err := conn.discord.ApplicationCommandDelete(command.ApplicationID, "", command.ID)
				if err != nil {
					slog.Warn("Failed to delete application command - ignoring cleaning of this command", slog.String("name", command.Name), slog.String("id", command.ID))
				}
			}
		}
	} else {
		slog.Warn("Failed to get application commands - skipping clean of old commands")
	}

	// Add a handler for all command interactions. The
	conn.discord.AddHandler(conn.handleCommandInvocation)

	slog.Info("Bot started")
	return conn, nil
}

// handleCommandInvocation handles a command being invocated.
func (c *Conn) handleCommandInvocation(session *discordgo.Session, event *discordgo.InteractionCreate) {
	commandName := event.ApplicationCommandData().Name
	slog.Debug("Got command request", slog.String("name", commandName))

	command, ok := c.commands[commandName]
	if !ok {
		slog.Warn("Got command interaction for unknown command", slog.String("name", commandName))
		return
	}

	// Acknowledge the command immediately. This will respond to the action that
	// the bot is "thinking". Later on, the response is updated with an
	// appropriate response after the command succeeds or fails.
	if err := session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		slog.Error("Failed to acknowledge command", slog.Any("error", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Invoke the appropriate action
	reply, err := command.Action(&Context{
		Context: ctx,
		session: session,
		event:   event,
	}, c)
	if err != nil {
		slog.Error("Failed to handle command", slog.Any("error", err))
		session.FollowupMessageCreate(event.Interaction, false, &discordgo.WebhookParams{
			Content: "An error occured. Try again in a little while.",
		})
	}

	// Update the response with the reply from the action
	session.FollowupMessageCreate(event.Interaction, false, &discordgo.WebhookParams{
		Content: reply,
	})
}

// Bot returns the Bot this connection is for.
func (c *Conn) Bot() *bot.Bot {
	return c.bot
}

// State returns the global state.
func (c *Conn) State() *state.State {
	return c.state
}

// Ready returns whether or not the
func (c *Conn) Ready() bool {
	return c.discord.DataReady
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.discord.Close()
}

// Play starts playing in the specified channel.
func (c *Conn) Play(guildID string, voiceChannelID string) {
	// TODO: Doesn't handle channel changes?
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
