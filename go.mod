module github.com/AlexGustafsson/clabbe

go 1.24.0

toolchain go1.24.3

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/caarlos0/env/v10 v10.0.0
	github.com/kkdai/youtube/v2 v2.10.4
	github.com/ollama/ollama v0.9.6
	github.com/pion/rtp v1.8.21
	github.com/pion/webrtc/v4 v4.1.3
	github.com/prometheus/client_golang v1.22.0
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/kkdai/youtube/v2 v2.10.4 => github.com/AlexGustafsson/youtube/v2 v2.10.5-0.20250511081928-46807c6833f4

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitly/go-simplejson v0.5.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/dop251/goja v0.0.0-20250309171923-bcd7cc6bf64c // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20250501235452-c0086092b71a // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
