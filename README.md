# Clabbe

A personal DJ that can be used locally or as a Discord bot. Uses AI to queue new
songs based on suggestions and recently played songs. Songs are fetched from
YouTube.

Clabbe is named after a Swedish DJ.

## Using

### Discord

The Clabbe bot exposes [slash commands](https://discord.com/blog/welcome-to-the-new-era-of-discord-apps?ref=badge)
in the Discord server it is invited to. These commands can be used to queue and
suggest music.

All commands require you to be in a voice channel.

#### /queue _query_

The queue command will search for a video on YouTube using the specified query.
The top match is added at the end of the queue.

If AI support is enabled and the interpolate option enabled (default), the bot
will fill the queue on its own once it's empty. It will do this by prioritizing
songs it has added when receiving suggestions (see /suggest). If no suggestions
have been added, it will try to play songs similar to recent listening history.

#### /suggest _query_ (AI)

If AI support is enabled, the suggest command can be used to ask an AI to play
music by an artist, music of a specific genre, vibe and so on. Suggestions will
be used by the bot once the queue is empty.

#### /playlist

The playlist command prints the current playlist.

#### /suggestions (AI)

The suggestions command prints the current suggestions.

#### /skip

The skip command skips the currently playing song.

#### /stop

The stop commands immediately disconnects the bot. The bot can be rejoined using
the /play command. Stopping the bot does not affect the queue or suggestions.

#### /play

The play command connects the bot to the voice channel you're in and requests it
to start playing songs from the queue.

## Running

### Discord

The bot needs a Discord bot token to run. The token can be specified in a config
file or as an environment variable. The config, queues and history are stored in
a configurable directory.

```yaml
discordBotToken: xxx
```

```shell
export DISCORD_BOT_TOKEN="xxx"
```

The bot can then be started like so.

```shell
./bot --config ./path/to/config/directory
```

## Development

### Building

Clabbe is written in Go. To build, Go 1.21 or later is required.

```shell
go build -o bot cmd/bot/main.go
```
