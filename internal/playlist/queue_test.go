package playlist

import (
	"testing"

	"github.com/jscyril/golang_music_player/api"
)

func TestQueueMovePreservesCurrentTrack(t *testing.T) {
	q := NewQueue()
	tracks := []*api.Track{
		{ID: "1", Title: "One"},
		{ID: "2", Title: "Two"},
		{ID: "3", Title: "Three"},
	}
	q.Set(tracks)
	if err := q.JumpTo(1); err != nil {
		t.Fatalf("JumpTo failed: %v", err)
	}

	if err := q.Move(1, 0); err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if got := q.Current(); got == nil || got.ID != "2" {
		t.Fatalf("current track should remain track 2, got %#v", got)
	}
	if q.Index() != 0 {
		t.Fatalf("current index should follow moved track, got %d", q.Index())
	}
}

func TestQueueMoveClearsShuffleState(t *testing.T) {
	q := NewQueue()
	q.Set([]*api.Track{
		{ID: "1", Title: "One"},
		{ID: "2", Title: "Two"},
		{ID: "3", Title: "Three"},
	})
	q.Shuffle()

	if err := q.Move(0, 1); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	if q.IsShuffled() {
		t.Fatal("manual queue move should clear shuffle mode")
	}
}

func TestQueueRemoveAdjustsCurrentIndex(t *testing.T) {
	q := NewQueue()
	q.Set([]*api.Track{
		{ID: "1", Title: "One"},
		{ID: "2", Title: "Two"},
		{ID: "3", Title: "Three"},
	})
	if err := q.JumpTo(2); err != nil {
		t.Fatalf("JumpTo failed: %v", err)
	}

	if err := q.Remove(0); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if got := q.Current(); got == nil || got.ID != "3" {
		t.Fatalf("current track should remain track 3, got %#v", got)
	}
	if q.Index() != 1 {
		t.Fatalf("current index should shift down, got %d", q.Index())
	}
}
