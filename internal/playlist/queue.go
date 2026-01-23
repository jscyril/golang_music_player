package playlist

import (
	"errors"
	"math/rand"
	"sync"

	"github.com/jscyril/golang_music_player/api"
)

// Queue represents a playback queue
type Queue struct {
	tracks     []*api.Track
	index      int
	repeatMode api.RepeatMode
	shuffle    bool
	original   []*api.Track // Original order before shuffle
	mu         sync.RWMutex
}

// NewQueue creates a new empty queue
func NewQueue() *Queue {
	return &Queue{
		tracks:     make([]*api.Track, 0),
		index:      0,
		repeatMode: api.RepeatNone,
		shuffle:    false,
	}
}

// Add adds tracks to the end of the queue
func (q *Queue) Add(tracks ...*api.Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append(q.tracks, tracks...)
}

// Set replaces the entire queue with new tracks
func (q *Queue) Set(tracks []*api.Track) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tracks = make([]*api.Track, len(tracks))
	copy(q.tracks, tracks)
	q.original = nil
	q.index = 0
}

// Clear removes all tracks from the queue
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tracks = make([]*api.Track, 0)
	q.original = nil
	q.index = 0
}

// Current returns the current track
func (q *Queue) Current() *api.Track {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.tracks) == 0 || q.index < 0 || q.index >= len(q.tracks) {
		return nil
	}
	return q.tracks[q.index]
}

// Next moves to the next track and returns it
func (q *Queue) Next() *api.Track {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tracks) == 0 {
		return nil
	}

	switch q.repeatMode {
	case api.RepeatOne:
		// Stay on current track
		return q.tracks[q.index]
	case api.RepeatAll:
		q.index = (q.index + 1) % len(q.tracks)
	default: // RepeatNone
		if q.index < len(q.tracks)-1 {
			q.index++
		} else {
			return nil // End of queue
		}
	}

	return q.tracks[q.index]
}

// Previous moves to the previous track and returns it
func (q *Queue) Previous() *api.Track {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tracks) == 0 {
		return nil
	}

	switch q.repeatMode {
	case api.RepeatOne:
		return q.tracks[q.index]
	case api.RepeatAll:
		q.index--
		if q.index < 0 {
			q.index = len(q.tracks) - 1
		}
	default:
		if q.index > 0 {
			q.index--
		}
	}

	return q.tracks[q.index]
}

// JumpTo jumps to a specific index
func (q *Queue) JumpTo(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if index < 0 || index >= len(q.tracks) {
		return errors.New("index out of bounds")
	}

	q.index = index
	return nil
}

// Remove removes a track at the specified index
func (q *Queue) Remove(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if index < 0 || index >= len(q.tracks) {
		return errors.New("index out of bounds")
	}

	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)

	// Adjust current index if needed
	if q.index > index {
		q.index--
	} else if q.index >= len(q.tracks) && len(q.tracks) > 0 {
		q.index = len(q.tracks) - 1
	}

	return nil
}

// Shuffle shuffles the queue (Fisher-Yates algorithm)
func (q *Queue) Shuffle() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tracks) <= 1 {
		return
	}

	// Save original order if not already shuffled
	if q.original == nil {
		q.original = make([]*api.Track, len(q.tracks))
		copy(q.original, q.tracks)
	}

	// Get current track to keep it at position 0
	currentTrack := q.tracks[q.index]

	// Shuffle all tracks
	n := len(q.tracks)
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		q.tracks[i], q.tracks[j] = q.tracks[j], q.tracks[i]
	}

	// Move current track to front
	for i, track := range q.tracks {
		if track.ID == currentTrack.ID {
			q.tracks[0], q.tracks[i] = q.tracks[i], q.tracks[0]
			break
		}
	}
	q.index = 0
	q.shuffle = true
}

// Unshuffle restores original order
func (q *Queue) Unshuffle() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.original == nil {
		return
	}

	// Find current track in original order
	currentTrack := q.tracks[q.index]
	q.tracks = q.original
	q.original = nil
	q.shuffle = false

	// Find new index of current track
	for i, track := range q.tracks {
		if track.ID == currentTrack.ID {
			q.index = i
			break
		}
	}
}

// SetRepeatMode sets the repeat mode
func (q *Queue) SetRepeatMode(mode api.RepeatMode) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.repeatMode = mode
}

// GetRepeatMode returns the current repeat mode
func (q *Queue) GetRepeatMode() api.RepeatMode {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.repeatMode
}

// IsShuffled returns whether the queue is shuffled
func (q *Queue) IsShuffled() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.shuffle
}

// GetAll returns a copy of all tracks in the queue
func (q *Queue) GetAll() []*api.Track {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*api.Track, len(q.tracks))
	copy(result, q.tracks)
	return result
}

// Len returns the number of tracks in the queue
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tracks)
}

// Index returns the current index
func (q *Queue) Index() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.index
}

// HasNext returns true if there's a next track
func (q *Queue) HasNext() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.repeatMode == api.RepeatAll || q.repeatMode == api.RepeatOne {
		return len(q.tracks) > 0
	}
	return q.index < len(q.tracks)-1
}

// HasPrevious returns true if there's a previous track
func (q *Queue) HasPrevious() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.repeatMode == api.RepeatAll || q.repeatMode == api.RepeatOne {
		return len(q.tracks) > 0
	}
	return q.index > 0
}
