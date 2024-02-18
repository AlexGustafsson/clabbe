package bot

import (
	"fmt"
	"strings"
	"time"
)

// Playlist
// If there are generated songs in the queue and it is suggested by a user
// directly, queue that song immediately instead

type PlaylistEntry struct {
	Title  string
	Source string
	// Added is the time at which the entry was added to the playlist.
	Added time.Time
	// ID is the YouTube video ID.
	ID string
}

func (e *PlaylistEntry) IsHigherPriority(other PlaylistEntry) bool {
	// TODO
	return false
}

// TODO: Keep mutex here instead, makes no sense to have it outside...
type Playlist struct {
	entries []PlaylistEntry
}

func NewPlaylist() *Playlist {
	return &Playlist{
		entries: make([]PlaylistEntry, 0),
	}
}

// Push the entry to the back of the playlist, then move it up to its
// appropriate position based on its priority.
func (p *Playlist) Push(entry PlaylistEntry) {
	p.entries = append(p.entries, entry)
	for i := len(p.entries) - 2; i >= 0; i-- {
		if p.entries[i+1].IsHigherPriority(p.entries[i]) {
			p.swap(i, i+1)
		}
	}
}

// PushFront pushes the entry to the front of the playlist, without taking its
// priority into account.
func (p *Playlist) PushFront(entry PlaylistEntry) {
	p.entries = append([]PlaylistEntry{entry}, p.entries...)
}

func (p *Playlist) Pop() (PlaylistEntry, bool) {
	if len(p.entries) > 0 {
		entry := p.entries[0]
		p.entries = p.entries[1:]
		return entry, true
	}

	return PlaylistEntry{}, false
}

// PeakN returns at most n entries from the front of the playlist
func (p *Playlist) PeakN(n int) []PlaylistEntry {
	entries := make([]PlaylistEntry, 0)
	for i := 0; i < n && i < len(p.entries); i++ {
		entries = append(entries, p.entries[i])
	}
	return entries
}

func (p *Playlist) String() string {
	var builder strings.Builder
	for i, entry := range p.entries {
		fmt.Fprintf(&builder, "%d. %s\n", i, entry.Title)
	}
	return builder.String()
}

// PopN returns at most n entries from the front of the playlist, removing them
// in the process.
func (p *Playlist) PopN(n int) []PlaylistEntry {
	entries := p.PeakN(n)
	p.entries = p.entries[len(entries):]
	return entries
}

func (p *Playlist) Clear() {
	p.entries = make([]PlaylistEntry, 0)
}

func (p *Playlist) swap(i int, j int) {
	p.entries[i], p.entries[j] = p.entries[j], p.entries[i]
}
