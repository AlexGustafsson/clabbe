module github.com/AlexGustafsson/clabbe

go 1.25

toolchain go1.25.5

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/caarlos0/env/v10 v10.0.0
	github.com/ollama/ollama v0.13.5
	github.com/pion/rtp v1.8.27
	github.com/pion/webrtc/v4 v4.1.8
	github.com/prometheus/client_golang v1.23.2
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/bwmarrin/discordgo => github.com/Richy-Z/discordgo v0.29.1-0.20251123191524-2672c0ec4dca

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
)
