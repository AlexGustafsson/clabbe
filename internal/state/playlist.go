package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/timeutil"
)

type Role string

const (
	RoleUser   Role = "user"
	RoleSystem Role = "system"
)

type Source string

const (
	SourceYouTube Source = "youtube"
)

type Entity struct {
	// Role is the role of the entity.
	Role Role `json:"role"`
	// ID is a role-specific id that uniquely identifies the entity.
	ID string `json:"id,omitempty"`
	// Name is a human-readable name of the entity.
	Name string `json:"name,omitempty"`
}

type PlaylistEntry struct {
	// Time is the time the entry was added to the playlist.
	Time time.Time `json:"time"`
	// Title is the title of the playlist.
	Title string `json:"title"`
	// AddedBy holds information on who added the entry to the playlist.
	AddedBy Entity `json:"addedBy"`
	// Source defines the source from which the entry can be found.
	Source Source `json:"source"`
	// URI is a source-specific URI that uniquely refers to the entry.
	URI string `json:"uri"`
}

type Playlist struct {
	mutex   sync.Mutex
	entries []PlaylistEntry
}

func NewPlaylist() *Playlist {
	return &Playlist{
		entries: make([]PlaylistEntry, 0),
	}
}

// CreatePlaylistIfNotExists makes sure that a playlist file exists. If it
// doesn't, it is created.
func CreatePlaylistIfNotExists(path string) error {
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return nil
	}

	playlist := NewPlaylist()
	return playlist.Store(path)
}

// ReadPlaylist reads a playlist file from the specified path.
func ReadPlaylist(path string) (*Playlist, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var playlist Playlist
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&playlist); err != nil {
		return nil, err
	}

	return &playlist, nil
}

// Store stores the playlist in the specified path.
// Writes are atomic.
func (p *Playlist) Store(path string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

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

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(p); err != nil {
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

// MarshalJSON implements json.Marshaler.
func (p *Playlist) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"version": "1",
		"entries": p.entries,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Playlist) UnmarshalJSON(data []byte) error {
	var values struct {
		Version string          `json:"version"`
		Entries []PlaylistEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	*p = Playlist{
		entries: values.Entries,
	}
	return nil
}

// AddEntry adds an entry to the playlist.
func (p *Playlist) AddEntry(entry PlaylistEntry) {
	p.entries = append(p.entries, entry)
}

// Push the entry to the back of the playlist.
func (p *Playlist) Push(entry PlaylistEntry) {
	p.entries = append(p.entries, entry)
}

// PushFront pushes the entry to the front of the playlist.
func (p *Playlist) PushFront(entry PlaylistEntry) {
	p.entries = append([]PlaylistEntry{entry}, p.entries...)
}

// Pop removes and returns the top entry.
func (p *Playlist) Pop() (PlaylistEntry, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.entries) > 0 {
		entry := p.entries[0]
		p.entries = p.entries[1:]
		return entry, true
	}

	return PlaylistEntry{}, false
}

// PopN returns at most n entries from the front of the playlist, removing them
// in the process.
func (p *Playlist) PopN(n int) []PlaylistEntry {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	entries := make([]PlaylistEntry, 0)
	for i := 0; i < n && i < len(p.entries); i++ {
		entries = append(entries, p.entries[i])
	}

	p.entries = p.entries[len(entries):]
	return entries
}

// PeakBackN returns at most n entries from the back of the playlist.
func (p *Playlist) PeakBackN(n int) []PlaylistEntry {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	entries := make([]PlaylistEntry, 0)
	for i := 0; i < n && i < len(p.entries); i++ {
		entries = append(entries, p.entries[len(p.entries)-1-i])
	}

	return entries
}

// Format formats the first n entries of the playlist using the specified format
// template.
//
// The template has the following values exposed:
//
//   - Index: index of the entry
//   - EntityName: name of the entity that added the entry
//   - Title: title of the entry
//   - RelativeTime: duration since the entry was added
func (p *Playlist) Format(format string, n int, reversed bool) (string, error) {
	t, err := template.New("").Parse(format)
	if err != nil {
		return "", err
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	var builder strings.Builder

	for i := 0; i < n && i < len(p.entries); i++ {
		entry := p.entries[i]
		if reversed {
			entry = p.entries[len(p.entries)-1-i]
		}

		entityName := entry.AddedBy.Name
		if entityName == "" && entry.AddedBy.Role == RoleSystem {
			entityName = "bot"
		} else if entityName == "" {
			entityName = "user"
		}

		err := t.Execute(&builder, map[string]any{
			"Index":        i + 1,
			"EntityName":   entityName,
			"Title":        entry.Title,
			"RelativeTime": timeutil.FormatRelativeDuration(-time.Since(entry.Time)),
		})
		if err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

// Clear clears the playlist.
func (p *Playlist) Clear() {
	p.entries = make([]PlaylistEntry, 0)
}
