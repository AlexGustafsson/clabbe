package discord

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	discord *discordgo.Session
}

func NewBot(token string) (*Bot, error) {
	bot := &Bot{}

	var err error
	bot.discord, err = discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	// bot.discord.AddHandler(bot.onReady)

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
					},
				},
			},
		},
	}

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

		if err := command.Handler(session, event); err != nil {
			slog.Error("Failed to handle command", slog.Any("error", err))
		}
	})

	return bot, nil
}

func (b *Bot) Start() error {
	return b.discord.Open()
}

func (b *Bot) Stop() error {
	return b.discord.Close()
}

func (b *Bot) handlePlayCommand(session *discordgo.Session, event *discordgo.InteractionCreate) error {
	return session.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Hello, World!",
		},
	})
}

// func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
// 	slog.Info("Bot is ready")
// }

// func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
// 	// Ignore all messages created by the bot itself
// 	if m.Author.ID == s.State.User.ID {
// 		return
// 	}

// 	if strings.HasPrefix(m.Content, "!play ") {

// 	}
// }
