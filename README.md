# Clabbe

![Banner containg usage examples](.github/banner.png)

A personal DJ that can be used locally or as a Discord bot. Optionally uses AI
to queue new songs based on suggestions and recently played songs. Songs are
fetched from YouTube.

Clabbe is named after a Swedish DJ.

## Features

- 🤖 Optionally uses AI to take requests and extrapolate new songs to play
- 🎹 Fetches songs from YouTube
- 🔥 Focus on performance. Uses about 10MB of RAM at runtime and virtually zero
  CPU. Other bots can use hundreds of megabytes of RAM and up to one CPU core.
- 🚀 Easy to setup. Zero-config other than bot token necessary for basic
  functionality
- 🛰️ Prometheus metrics, liveliness and readiness probe support for maintainable
  deployments using Docker or Kubernetes.

## Using

### Discord

The Clabbe bot exposes [slash commands](https://discord.com/blog/welcome-to-the-new-era-of-discord-apps?ref=badge)
in the Discord server it is invited to. These commands can be used to queue and
suggest music.

#### `/queue <query>`

The queue command will search for a video on YouTube using the specified query.
The top match is added at the end of the queue.

If AI support is enabled and the extrapolate option enabled (default), the bot
will fill the queue on its own once it's empty. It will do this by prioritizing
songs it has added when receiving suggestions (see /suggest). If no suggestions
have been added, it will try to play songs similar to recent listening history.

This command requires you to be in a voice channel.

#### `/suggest <query>` (AI)

If AI support is enabled, the suggest command can be used to ask an AI to play
music by an artist, music of a specific genre, vibe and so on. Suggestions will
be used by the bot once the queue is empty.

The bot should be capable of a wide variety of suggestions. Here's a few
examples that typically work well:

- most played songs 1986
- songs about dogs
- famous guitar riffs
- 90s music quiz
- famous cover songs
- rock & roll duets

This command requires you to be in a voice channel.

#### `/queued`

The queued command prints the list of queued songs.

#### `/suggestions` (AI)

The suggestions command prints the current suggestions.

#### `/recent`

The recent command prints recently played songs.

#### `/skip [n]`

The skip command skips the currently playing song. If `n` is specified, the bot
will skip the specified number of songs.

#### `/stop`

The stop commands immediately disconnects the bot. The bot can be rejoined using
the /play command. Stopping the bot does not affect the queue or suggestions.

#### `/play`

The play command connects the bot to the voice channel you're in and requests it
to start playing songs from the queue.

This command requires you to be in a voice channel.

## Running

### Discord

The bot needs a Discord bot token to run. The token can be specified in a config
file or as an environment variable. The config, queues and history are stored in
a configurable directory specified when the bot is started.

```yaml
discordBotToken: xxx
```

```shell
export DISCORD_BOT_TOKEN="xxx"
```

See `config.yaml` for an example config file, with the default values set.

The bot can be started on the host or using Docker.

```shell
./bot --config ./config
```

```shell
docker run ghrc.io/alexgustafsson/clabbe --config ./config
```

## Development

Clabbe is written in Go. It's meant to be small and fast rather than fully
featured, big and slow.

The code base consists of the following notable parts:

- `internal/bot` - a platform agnostic bot. Exposes core bot functionality.
- `internal/discord` - a Discord adapter for the bot.
  - `internal/discord/conn.go` - A connection abstraction.
  - `internal/discord/commands.go` - Available command definitions.
  - `internal/discord/actions.go` - Actions called when invoking commands.
- `internal/ebml`, `internal/webm` - a webm demuxer in order to stream opus
  samples immediately from a source to Discord.
- `internal/ffmpeg` - an ffmpeg abstraction to play audio using ffplay.
  Currently only used for development tools.
- `internal/llm` - LLM abstraction, ollama client.
- `internal/state` - state management.
- `internal/streaming/youtube` - abstractions and implementations for searching
  for videos on YouTube.
- `internal/ytdlb` - yt-dlp wrapper.

### Building

Clabbe is written in Go and meant to run in Docker.

```shell
docker build -t ghcr.io/alexgustafsson/clabbe .
```

### Use of LLM

The bot can use Open AI's APIs to take suggestions and to recommend more music.
In order to be able to use music newer than what's "memorized" by the LLM and in
order to promote playing new songs, the LLM receives a prompt containing the
following parts.

- Introduction / context
- History
- Similar songs
- Query
