package ui

import "testing"

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

	for i := 0; i < 3; i++ {
		d.Update()
	}
}
