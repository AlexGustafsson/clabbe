package state

import (
	"os"
	"path"
)

// State holds application state.
type State struct {
	Config *Config

	queuePath string
	Queue     *Playlist

	suggestionsPath string
	Suggestions     *Playlist

	historyPath string
	History     *Playlist
}

// LoadOrInit loads the state from the specified base path.
// If the state is not initialized (i.e. config files etc. not created), the
// state is initialized.
func LoadOrInit(basePath string) (*State, error) {
	configPath := path.Join(basePath, "config.yaml")
	queuePath := path.Join(basePath, "queue.json")
	suggestionsPath := path.Join(basePath, "suggestions.json")
	historyPath := path.Join(basePath, "history.json")

	if err := os.MkdirAll(basePath, os.ModePerm); err != nil {
		return nil, err
	}

	if err := CreateConfigIfNotExists(configPath); err != nil {
		return nil, err
	}
	config, err := ReadConfig(configPath)
	if err != nil {
		return nil, err
	}
	if err := config.PopulateFromEnvironment(); err != nil {
		return nil, err
	}

	if err := CreatePlaylistIfNotExists(queuePath); err != nil {
		return nil, err
	}
	queue, err := ReadPlaylist(queuePath)
	if err != nil {
		return nil, err
	}

	if err := CreatePlaylistIfNotExists(suggestionsPath); err != nil {
		return nil, err
	}
	suggestions, err := ReadPlaylist(suggestionsPath)
	if err != nil {
		return nil, err
	}

	if err := CreatePlaylistIfNotExists(historyPath); err != nil {
		return nil, err
	}
	history, err := ReadPlaylist(historyPath)
	if err != nil {
		return nil, err
	}

	return &State{
		Config: config,

		queuePath: queuePath,
		Queue:     queue,

		suggestionsPath: suggestionsPath,
		Suggestions:     suggestions,

		historyPath: historyPath,
		History:     history,
	}, nil
}

// Store stores the state to the same location it was read from.
func (s *State) Store() error {
	if err := s.Queue.Store(s.queuePath); err != nil {
		return err
	}

	if err := s.Suggestions.Store(s.suggestionsPath); err != nil {
		return err
	}

	if err := s.History.Store(s.historyPath); err != nil {
		return err
	}

	return nil
}
