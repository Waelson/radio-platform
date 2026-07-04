package queue_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/queue"
)

// --- helpers -----------------------------------------------------------------

func spotInput(title string) commands.QueueItemInput {
	return commands.QueueItemInput{
		AssetID:    "asset_" + title,
		Path:       "/audio/" + title + ".mp3",
		Type:       string(queue.AssetTypeSpot),
		Title:      title,
		DurationMS: 30000,
	}
}

func jingleInput(title string) commands.QueueItemInput {
	return commands.QueueItemInput{
		AssetID:    "asset_" + title,
		Path:       "/audio/" + title + ".mp3",
		Type:       string(queue.AssetTypeJingle),
		Title:      title,
		DurationMS: 8000,
	}
}

func enqueueBreakCmd(b commands.BreakItemInput) commands.Command {
	return commands.New(commands.CmdEnqueueBreak, commands.EnqueueBreakPayload{Break: b})
}

// --- HandleEnqueueBreak — expansion ------------------------------------------

func TestHandleEnqueueBreak_ExpandsFlat_SpotsOnly(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco 14h",
		Spots: []commands.QueueItemInput{
			spotInput("spot_a"),
			spotInput("spot_b"),
			spotInput("spot_c"),
		},
	})

	if err := mgr.HandleEnqueueBreak(context.Background(), cmd); err != nil {
		t.Fatalf("HandleEnqueueBreak error: %v", err)
	}

	if mgr.Size() != 3 {
		t.Fatalf("Size = %d, want 3", mgr.Size())
	}

	pending := mgr.ListPending()
	breakID := pending[0].BreakID
	if breakID == "" {
		t.Fatal("BreakID is empty on first spot")
	}

	for i, it := range pending {
		if it.BreakID != breakID {
			t.Errorf("item[%d].BreakID = %q, want %q", i, it.BreakID, breakID)
		}
		if it.BreakTitle != "Bloco 14h" {
			t.Errorf("item[%d].BreakTitle = %q, want %q", i, it.BreakTitle, "Bloco 14h")
		}
		if it.BreakTotal != 3 {
			t.Errorf("item[%d].BreakTotal = %d, want 3", i, it.BreakTotal)
		}
		if it.BreakSeq != i+1 {
			t.Errorf("item[%d].BreakSeq = %d, want %d", i, it.BreakSeq, i+1)
		}
		if it.BreakRole != "spot" {
			t.Errorf("item[%d].BreakRole = %q, want spot", i, it.BreakRole)
		}
	}
}

func TestHandleEnqueueBreak_WithOpenAndClose(t *testing.T) {
	mgr, _ := newManager(t)

	open := jingleInput("open_jingle")
	close := jingleInput("close_jingle")
	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco Completo",
		Open:  &open,
		Spots: []commands.QueueItemInput{
			spotInput("spot_1"),
			spotInput("spot_2"),
		},
		Close: &close,
	})

	if err := mgr.HandleEnqueueBreak(context.Background(), cmd); err != nil {
		t.Fatalf("HandleEnqueueBreak error: %v", err)
	}

	pending := mgr.ListPending()
	if len(pending) != 4 {
		t.Fatalf("len(pending) = %d, want 4", len(pending))
	}

	cases := []struct {
		role string
		seq  int
	}{
		{"open", 1},
		{"spot", 2},
		{"spot", 3},
		{"close", 4},
	}

	breakID := pending[0].BreakID
	for i, tc := range cases {
		it := pending[i]
		if it.BreakRole != tc.role {
			t.Errorf("item[%d].BreakRole = %q, want %q", i, it.BreakRole, tc.role)
		}
		if it.BreakSeq != tc.seq {
			t.Errorf("item[%d].BreakSeq = %d, want %d", i, it.BreakSeq, tc.seq)
		}
		if it.BreakTotal != 4 {
			t.Errorf("item[%d].BreakTotal = %d, want 4", i, it.BreakTotal)
		}
		if it.BreakID != breakID {
			t.Errorf("item[%d].BreakID mismatch", i)
		}
	}
}

// --- Transition rules --------------------------------------------------------

func TestHandleEnqueueBreak_OpenGetsCrossfade(t *testing.T) {
	mgr, _ := newManager(t)

	open := jingleInput("open")
	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco",
		Open:  &open,
		Spots: []commands.QueueItemInput{spotInput("s1")},
	})
	_ = mgr.HandleEnqueueBreak(context.Background(), cmd)

	pending := mgr.ListPending()
	if pending[0].Transition.Type != queue.TransitionCrossfade {
		t.Errorf("Open transition = %q, want CROSSFADE", pending[0].Transition.Type)
	}
	if pending[0].Transition.DurationMS != 3000 {
		t.Errorf("Open transition DurationMS = %d, want 3000", pending[0].Transition.DurationMS)
	}
	if pending[1].Transition.Type != queue.TransitionCut {
		t.Errorf("Spot[0] transition = %q, want CUT", pending[1].Transition.Type)
	}
}

func TestHandleEnqueueBreak_NoOpen_FirstSpotGetsCrossfade(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco",
		Spots: []commands.QueueItemInput{
			spotInput("s1"),
			spotInput("s2"),
		},
	})
	_ = mgr.HandleEnqueueBreak(context.Background(), cmd)

	pending := mgr.ListPending()
	if pending[0].Transition.Type != queue.TransitionCrossfade {
		t.Errorf("First spot transition = %q, want CROSSFADE", pending[0].Transition.Type)
	}
	if pending[1].Transition.Type != queue.TransitionCut {
		t.Errorf("Second spot transition = %q, want CUT", pending[1].Transition.Type)
	}
}

func TestHandleEnqueueBreak_CloseGetsCut(t *testing.T) {
	mgr, _ := newManager(t)

	cl := jingleInput("close")
	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco",
		Spots: []commands.QueueItemInput{spotInput("s1")},
		Close: &cl,
	})
	_ = mgr.HandleEnqueueBreak(context.Background(), cmd)

	pending := mgr.ListPending()
	// pending[0] = spot (crossfade), pending[1] = close (cut)
	if pending[1].Transition.Type != queue.TransitionCut {
		t.Errorf("Close transition = %q, want CUT", pending[1].Transition.Type)
	}
	if pending[1].BreakRole != "close" {
		t.Errorf("Close BreakRole = %q, want close", pending[1].BreakRole)
	}
}

// --- Validation --------------------------------------------------------------

func TestHandleEnqueueBreak_EmptySpots_ReturnsError(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco Vazio",
		Spots: nil,
	})
	err := mgr.HandleEnqueueBreak(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for empty spots, got nil")
	}
}

// --- Multiple breaks have distinct IDs --------------------------------------

func TestHandleEnqueueBreak_TwoBreaks_DifferentIDs(t *testing.T) {
	mgr, _ := newManager(t)

	cmd1 := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco 1",
		Spots: []commands.QueueItemInput{spotInput("s1")},
	})
	cmd2 := enqueueBreakCmd(commands.BreakItemInput{
		Title: "Bloco 2",
		Spots: []commands.QueueItemInput{spotInput("s2")},
	})

	_ = mgr.HandleEnqueueBreak(context.Background(), cmd1)
	_ = mgr.HandleEnqueueBreak(context.Background(), cmd2)

	pending := mgr.ListPending()
	if len(pending) != 2 {
		t.Fatalf("len(pending) = %d, want 2", len(pending))
	}
	if pending[0].BreakID == pending[1].BreakID {
		t.Errorf("both breaks share the same BreakID %q", pending[0].BreakID)
	}
}

// --- ListPending -------------------------------------------------------------

func TestListPending_DoesNotIncludeCurrent(t *testing.T) {
	mgr, _ := newManager(t)
	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	mgr.PopAsCurrent()

	pending := mgr.ListPending()
	if len(pending) != 1 {
		t.Fatalf("ListPending len = %d, want 1", len(pending))
	}
	if pending[0].Title != "B" {
		t.Errorf("ListPending[0].Title = %q, want B", pending[0].Title)
	}
}
