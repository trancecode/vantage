package asset

import "testing"

func TestDefaultFontsLoaded(t *testing.T) {
	if DefaultProportionalFont == nil {
		t.Fatal("DefaultProportionalFont is nil")
	}
	if DefaultMonospaceFont == nil {
		t.Fatal("DefaultMonospaceFont is nil")
	}
}
