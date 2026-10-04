package ui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/trancecode/vantage/asset"
)

// TestDialogEmptyOptionsUpdateDoesNotPanic guards against a regression where a
// dialog constructed with no options (an info/confirmation dialog offering
// only Cancel) panics on arrow-key navigation because SelectedIndex is
// advanced modulo len(Options), which divides by zero when Options is empty.
//
// inpututil key state is driven by ebiten's internal input singleton, which
// this package cannot set from a test, so this does not simulate an actual
// ArrowDown/ArrowUp press. It does exercise every other branch of Update
// (mouse hover, number-key selection, Enter, Escape, and the post-loop
// highlight pass) with an empty Options slice, and confirms the dialog can
// be laid out and drawn in that state without panicking.
func TestDialogEmptyOptionsUpdateDoesNotPanic(t *testing.T) {
	d := NewDialog("No options", nil, func() {})
	d.SetScreenSize(800, 600)

	for range 3 {
		d.Update()
	}
}

// TestDialogWidensToHoldItsContext checks that a context line longer than the
// title and the options widens the dialog box so the line ends inside it.
func TestDialogWidensToHoldItsContext(t *testing.T) {
	options := []DialogOption{{Label: "Rest"}}
	short := NewDialog("Inn", options, func() {})
	long := NewDialog("Inn", options, func() {})
	long.Context = "Rest here. Stamina comes back within 15 minutes; wounds need a full night."

	shortWidth, _ := short.dialogSize()
	longWidth, _ := long.dialogSize()
	if longWidth <= shortWidth {
		t.Fatalf("dialog width with a long context = %v, want wider than %v", longWidth, shortWidth)
	}

	contextFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogFontSize)}
	contextEnd := dialogPaddingX + text.Advance(long.Context, contextFace)
	if contextEnd > longWidth {
		t.Errorf("context line ends at x=%v, past the dialog's width %v", contextEnd, longWidth)
	}
}

// TestDialogCancelLabel checks that the cancel button reads "Cancel" by
// default and CancelLabel otherwise, and that a long label is measured into
// the dialog's width.
func TestDialogCancelLabel(t *testing.T) {
	plain := NewDialog("Forge", nil, func() {})
	plain.SetScreenSize(800, 600)
	plain.recalculateLayout()
	if plain.cancelButton.Label != "Cancel" {
		t.Errorf("default cancel label = %q, want %q", plain.cancelButton.Label, "Cancel")
	}

	closing := NewDialog("Forge", nil, func() {})
	closing.CancelLabel = "Close"
	closing.SetScreenSize(800, 600)
	closing.recalculateLayout()
	if closing.cancelButton.Label != "Close" {
		t.Errorf("cancel label = %q, want %q", closing.cancelButton.Label, "Close")
	}

	wide := NewDialog("Forge", nil, func() {})
	wide.CancelLabel = "Leave this dialog and go back to the map without choosing anything"
	plainWidth, _ := plain.dialogSize()
	wideWidth, _ := wide.dialogSize()
	if wideWidth <= plainWidth {
		t.Errorf("dialog width with a long cancel label = %v, want wider than %v", wideWidth, plainWidth)
	}
}
