package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	*discordgo.ApplicationCommand
	Handler func(context.Context, *discordgo.Session, *discordgo.InteractionCreate) error
}
