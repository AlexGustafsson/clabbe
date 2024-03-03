package discord

import (
	"context"
	"fmt"

	"github.com/AlexGustafsson/clabbe/internal/state"
	"github.com/bwmarrin/discordgo"
)

// Context holds the context of a command invocation.
type Context struct {
	context.Context
	session *discordgo.Session
	event   *discordgo.InteractionCreate
}

var (
	ErrNotInVoiceChannel = fmt.Errorf("user is not in a voice channel")
)

// VoiceChannel returns the guild id and voice channel id of the voice channel
// the requesting user is currently in.
// Returns ErrNotInVoiceChannel if the user is not in a voice channel.
func (c *Context) VoiceChannel() (string, string, error) {
	// Find the channel that the message came from.
	channel, err := c.session.State.Channel(c.event.ChannelID)
	if err != nil {
		return "", "", fmt.Errorf("failed to identify event's channel")
	}

	// Find the guild for that channel.
	guild, err := c.session.State.Guild(channel.GuildID)
	if err != nil {
		return "", "", fmt.Errorf("failed to identify event's guild")
	}

	// Find the voice channel of the sender
	voiceChannelID := ""
	for _, voiceStates := range guild.VoiceStates {
		if voiceStates.UserID == c.event.Member.User.ID {
			voiceChannelID = voiceStates.ChannelID
			break
		}
	}

	if voiceChannelID == "" {
		return "", "", ErrNotInVoiceChannel
	}

	return guild.ID, voiceChannelID, nil
}

// String returns a string parameter by key.
func (c *Context) String(key string) (string, bool) {
	options := c.event.ApplicationCommandData().Options

	for _, option := range options {
		if option.Name == key {
			return option.StringValue(), true
		}
	}

	return "", false
}

// Number returns a number parameter by key.
func (c *Context) Number(key string) (float64, bool) {
	options := c.event.ApplicationCommandData().Options

	for _, option := range options {
		if option.Name == key {
			return option.FloatValue(), true
		}
	}

	return 0, false
}

// Boolean returns a boolean parameter by key.
func (c *Context) Boolean(key string) (bool, bool) {
	options := c.event.ApplicationCommandData().Options

	for _, option := range options {
		if option.Name == key {
			return option.BoolValue(), true
		}
	}

	return false, false
}

// Entity returns the entity that invoked the command.
func (c *Context) Entity() state.Entity {
	name := c.event.Member.Nick
	if name == "" {
		name = c.event.Member.User.Username
	}

	return state.Entity{
		Role: state.RoleUser,
		ID:   fmt.Sprintf("%s/%s", c.event.GuildID, c.event.Member.User.ID),
		Name: name,
	}
}
