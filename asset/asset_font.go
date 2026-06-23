package asset

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	//go:embed font/google-sans-flex/GoogleSansFlex.ttf
	googleSansFlexTTF []byte

	//go:embed font/google-sans-code/GoogleSansCode.ttf
	googleSansCodeTTF []byte
)

// DefaultProportionalFont is the engine's default proportional font
// (Google Sans Flex, OFL), for general UI and world-space text.
var DefaultProportionalFont = MustLoadFont(googleSansFlexTTF)

// DefaultMonospaceFont is the engine's default monospace font
// (Google Sans Code, OFL), for debug overlays and aligned columnar text.
var DefaultMonospaceFont = MustLoadFont(googleSansCodeTTF)

// LoadFont parses TrueType/OpenType font bytes into a GoTextFaceSource.
func LoadFont(b []byte) (*text.GoTextFaceSource, error) {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("creating font source: %w", err)
	}
	return s, nil
}

// MustLoadFont parses font bytes and panics on failure, for package
// initialization of embedded fonts.
func MustLoadFont(b []byte) *text.GoTextFaceSource {
	s, err := LoadFont(b)
	if err != nil {
		panic(fmt.Sprintf("loading font: %v", err))
	}
	return s
}
