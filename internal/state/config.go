package state

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v10"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DiscordBotToken string `yaml:"discordBotToken,omitempty" env:"DISCORD_BOT_TOKEN"`
	OpenAIKey       string `yaml:"openAiApiKey,omitempty" env:"OPENAI_API_KEY"`

	ExtrapolateWhenEmpty  bool `yaml:"extrapolateWhenEmpty"`
	ExtrapolationLookback int  `yaml:"extrapolationLookback"`

	Prometheus *PrometheusConfig `yaml:"prometheus,omitempty"`

	Prompt       string `yaml:"-"`
	ThemesPrompt string `yaml:"-"`

	LogLevel slog.Level `yaml:"logLevel"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    uint16 `yaml:"port"`
}

// DefaultConfig returns the default config.
func DefaultConfig() *Config {
	return &Config{
		ExtrapolateWhenEmpty:  true,
		ExtrapolationLookback: 10,

		Prometheus: &PrometheusConfig{
			Enabled: false,
			Port:    8080,
		},

		Prompt:       DefaultPrompt,
		ThemesPrompt: GenerateThemesPrompt,

		LogLevel: slog.LevelInfo,
	}
}

// PopulateFromEnvironment populates the config with values from environment
// variables.
func (c *Config) PopulateFromEnvironment() error {
	return env.Parse(c)
}

// CreateConfigIfNotExists makes sure that a config file exists. If it doesn't,
// it is created and populated with the default config.
func CreateConfigIfNotExists(path string) error {
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return nil
	}

	config := DefaultConfig()
	return config.Store(path)
}

// ReadConfig reads a config file from the specified path.
func ReadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return config, err
}

// Store stores the config in the specified path.
// Writes are atomic.
func (c *Config) Store(path string) error {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()

	encoder := yaml.NewEncoder(file)
	if err := encoder.Encode(&c); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
