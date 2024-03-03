package discord

import (
	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/state"
)

// Command exposes functionality as a Discord command.
type Command struct {
	Name        string
	Description string
	Options     []Option
	// Action is the function to invoke whenever the command is invoked.
	// The action returns the response as a string, or an error if an unexpected
	// failure occurs.
	Action func(*Context, *Conn) (string, error)
	// EnabledFunc returns true if the command is enabled.
	// A nil EnabledFunc implicitly enables the command.
	EnabledFunc func(*state.State, *bot.Bot) bool
}

// Option defines an option to a command.
type Option struct {
	Name        string
	Description string
	// Type defaults to string.
	Type     OptionType
	Required bool
	// EnabledFunc returns true if the option is enabled.
	// A nil EnabledFunc implicitly enables the option.
	EnableFunc func(*state.State, *bot.Bot) bool
}

type OptionType int

const (
	OptionTypeString = iota << 1
	OptionTypeNumber
	OptionTypeBoolean
)

// TODO:
// /queue add xxx
// /queue print
// /queue clear
// /suggestions add xxx
// /suggestions print
// /suggestions clear
// /recent print

// commands holds all available commands.
var commands = []Command{
	{
		Name:        "play",
		Description: "Start playing music in your voice channel",
		Action:      PlayAction,
		Options: []Option{
			{
				Name:        "auto",
				Description: "auto play using AI suggestions",
				Type:        OptionTypeBoolean,
				EnableFunc: func(s *state.State, b *bot.Bot) bool {
					return b.OpenAIEnabled()
				},
			},
		},
	},
	{
		Name:        "queue",
		Description: "Queue music to your voice channel",
		Action:      QueueAction,
		Options: []Option{
			{
				Name:        "query",
				Description: "YouTube search query",
				Required:    true,
			},
		},
	},
	{
		Name:        "queued",
		Description: "Print queue",
		Action:      QueuedAction,
	},
	{
		Name:        "suggest",
		Description: "Suggest music to your voice channel",
		Action:      SuggestAction,
		Options: []Option{
			{
				Name:        "query",
				Description: "LLM search query",
				Required:    true,
			},
		},
		EnabledFunc: func(s *state.State, b *bot.Bot) bool {
			return b.OpenAIEnabled()
		},
	},
	{
		Name:        "suggestions",
		Description: "Print suggestions",
		Action:      SuggestionsAction,
		EnabledFunc: func(s *state.State, b *bot.Bot) bool {
			return b.OpenAIEnabled()
		},
	},
	{
		Name:        "recent",
		Description: "Print recently played songs",
		Action:      RecentAction,
	},
	{
		Name:        "stop",
		Description: "Disconnect the bot",
		Action:      StopAction,
	},
	{
		Name:        "skip",
		Description: "Skip the current song",
		Action:      SkipAction,
		Options: []Option{
			{
				Name:        "n",
				Description: "Number of songs to skip",
				Type:        OptionTypeNumber,
			},
		},
	},
}
