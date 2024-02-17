package discord

import "github.com/bwmarrin/discordgo"

type Command struct {
	*discordgo.ApplicationCommand
	Handler func(*discordgo.Session, *discordgo.InteractionCreate) error
}
