package discord

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexGustafsson/clabbe/internal/bot"
	"github.com/AlexGustafsson/clabbe/internal/streaming/youtube"
)

func PlayAction(ctx *Context, conn *Conn) (string, error) {
	guildID, voiceChannelID, err := ctx.VoiceChannel()
	if err == ErrNotInVoiceChannel {
		return "You must be in a voice channel to do that", nil
	} else if err != nil {
		return "", err
	}

	auto, ok := ctx.Boolean("auto")
	if ok && auto {
		b := conn.Bot()
		b.ExtrapolationType = bot.ExtrapolationTypeSuggest
	}

	conn.Play(guildID, voiceChannelID)

	currentEntry := conn.Bot().NowPlaying()
	if currentEntry != nil {
		return fmt.Sprintf("Currently playing **%s**", currentEntry.Title), nil
	}

	return "On my way!", nil
}

func QueueAction(ctx *Context, conn *Conn) (string, error) {
	guildID, voiceChannelID, err := ctx.VoiceChannel()
	if err == ErrNotInVoiceChannel {
		return "You must be in a voice channel to do that", nil
	} else if err != nil {
		return "", err
	}

	query, ok := ctx.String("query")
	if !ok {
		return "Missing required query parameter", nil
	}

	entries, err := conn.Bot().Queue(ctx, query, ctx.Entity(), nil)
	if err == youtube.ErrTooManyRequests {
		return "Too many requests made to YouTube. Try again in a short while", nil
	} else if err != nil {
		slog.Error("Failed to queue query results", slog.Any("error", err))
		return "I can't do that right now. Try again in a short while", nil
	}

	if len(entries) == 0 {
		return "I couldn't find anything for you", nil
	}

	conn.Play(guildID, voiceChannelID)

	return fmt.Sprintf("Queued **%s**", entries[0].Title), nil
}

func SuggestAction(ctx *Context, conn *Conn) (string, error) {
	guildID, voiceChannelID, err := ctx.VoiceChannel()
	if err == ErrNotInVoiceChannel {
		return "You must be in a voice channel to do that", nil
	} else if err != nil {
		return "", err
	}

	query, ok := ctx.String("query")
	if !ok {
		return "Missing required query parameter", nil
	}

	entries, err := conn.Bot().Suggest(ctx, ctx.Entity(), query)
	if err == youtube.ErrTooManyRequests {
		return "Too many requests made to YouTube. Try again in a short while", nil
	} else if err != nil {
		slog.Error("Failed to suggest results", slog.Any("error", err))
		return "I can't do that right now. Try again in a short while", nil
	}

	if len(entries) == 0 {
		return "I couldn't find anything for you", nil
	}

	conn.Play(guildID, voiceChannelID)

	var response strings.Builder
	response.WriteString("I've added these songs to the list of suggestions.\n")
	for i, entry := range entries {
		fmt.Fprintf(&response, "%d. **%s**\n", i+1, entry.Title)
	}
	return response.String(), nil
}

func QueuedAction(ctx *Context, conn *Conn) (string, error) {
	format := "{{.Index}}. {{.EntityName}} queued {{.RelativeTime}} - **{{.Title}}**\n"
	contents, err := conn.State().Queue.Format(format, 20, false)
	if err != nil {
		return "", err
	}
	if contents == "" {
		contents = "No songs"
	}
	return contents, nil
}

func SuggestionsAction(ctx *Context, conn *Conn) (string, error) {
	format := "{{.Index}}. **{{.Title}}**\n"
	contents, err := conn.State().Suggestions.Format(format, 20, false)
	if err != nil {
		return "", err
	}
	if contents == "" {
		contents = "No songs"
	}
	return contents, nil
}

func RecentAction(ctx *Context, conn *Conn) (string, error) {
	format := "{{.Index}}. {{.EntityName}} played {{.RelativeTime}} - **{{.Title}}**\n"
	contents, err := conn.State().History.Format(format, 20, true)
	if err != nil {
		return "", err
	}
	if contents == "" {
		contents = "No songs"
	}
	return contents, nil
}

func StopAction(ctx *Context, conn *Conn) (string, error) {
	conn.Bot().Stop()
	return "Stopping", nil
}

func SkipAction(ctx *Context, conn *Conn) (string, error) {
	n, _ := ctx.Number("n")
	conn.Bot().SkipN(int(n))
	return "Skipping", nil
}
