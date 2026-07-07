package visualtest

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MismatchKind classifies why two images differ.
type MismatchKind int

const (
	// MismatchKindNone is the zero value and means no mismatch.
	MismatchKindNone MismatchKind = iota
	// MismatchKindSize means the images have different dimensions; no pixels
	// were compared.
	MismatchKindSize
	// MismatchKindPixel means the images are the same size but a pixel differs.
	MismatchKindPixel
)

// Mismatch describes the first difference found between two images. It is
// returned only when a difference exists (a nil *Mismatch means the images are
// identical) and implements error so a comparison failure reads naturally as an
// error while still exposing the coordinates and colors for inspection.
type Mismatch struct {
	// Kind classifies the difference.
	Kind MismatchKind

	// WantBounds is the golden image's bounds.
	WantBounds image.Rectangle

	// GotBounds is the candidate image's bounds. It differs from WantBounds
	// when Kind is MismatchKindSize.
	GotBounds image.Rectangle

	// X is the x coordinate of the first differing pixel, relative to the
	// images' top-left corner (Kind == MismatchKindPixel).
	X int

	// Y is the y coordinate of the first differing pixel, relative to the
	// images' top-left corner (Kind == MismatchKindPixel).
	Y int

	// Want is the golden color at (X, Y) (Kind == MismatchKindPixel).
	Want color.RGBA

	// Got is the candidate color at (X, Y) (Kind == MismatchKindPixel).
	Got color.RGBA
}

// Error returns a human-readable reason for the mismatch.
func (m *Mismatch) Error() string {
	switch m.Kind {
	case MismatchKindSize:
		return fmt.Sprintf("size mismatch: want %dx%d, got %dx%d",
			m.WantBounds.Dx(), m.WantBounds.Dy(), m.GotBounds.Dx(), m.GotBounds.Dy())
	case MismatchKindPixel:
		return fmt.Sprintf("pixel mismatch at (%d, %d): want %v, got %v", m.X, m.Y, m.Want, m.Got)
	default:
		return "no mismatch"
	}
}

// CompareImages compares want and got pixel-for-pixel and returns the first
// difference, or nil if they are identical. It first checks the dimensions,
// then scans in row-major order so a reported pixel mismatch is always the
// top-most, left-most differing pixel. Colors are normalized to RGBA before
// comparison, so images with different underlying color models but identical
// visible pixels compare equal.
func CompareImages(want, got image.Image) *Mismatch {
	wb, gb := want.Bounds(), got.Bounds()
	if wb.Dx() != gb.Dx() || wb.Dy() != gb.Dy() {
		return &Mismatch{Kind: MismatchKindSize, WantBounds: wb, GotBounds: gb}
	}

	for y := range wb.Dy() {
		for x := range wb.Dx() {
			w := rgbaAt(want, wb.Min.X+x, wb.Min.Y+y)
			g := rgbaAt(got, gb.Min.X+x, gb.Min.Y+y)
			if w != g {
				return &Mismatch{
					Kind:       MismatchKindPixel,
					WantBounds: wb,
					GotBounds:  gb,
					X:          x,
					Y:          y,
					Want:       w,
					Got:        g,
				}
			}
		}
	}
	return nil
}

// rgbaAt returns the color at (x, y) normalized to color.RGBA.
func rgbaAt(img image.Image, x, y int) color.RGBA {
	return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
}

// ComparePNGFiles decodes the PNG files at wantPath and gotPath and compares
// them with [CompareImages]. The returned error covers read and decode
// failures; a content difference is reported through the returned *Mismatch
// (nil when the images are identical).
func ComparePNGFiles(wantPath, gotPath string) (*Mismatch, error) {
	want, err := decodePNG(wantPath)
	if err != nil {
		return nil, fmt.Errorf("reading golden %q: %w", wantPath, err)
	}
	got, err := decodePNG(gotPath)
	if err != nil {
		return nil, fmt.Errorf("reading candidate %q: %w", gotPath, err)
	}
	return CompareImages(want, got), nil
}

// decodePNG reads and decodes the PNG file at path.
func decodePNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err // os.PathError already includes operation and filename
	}
	defer func() { _ = file.Close() }()

	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}
	return img, nil
}

// SequenceMismatch reports the first difference between two PNG sequences,
// which is either a differing frame or the sequences having different lengths.
// A nil *SequenceMismatch means the sequences are identical. It implements
// error.
type SequenceMismatch struct {
	// Index is the 0-based frame position where the difference occurs.
	Index int

	// WantPath is the golden frame at Index, or empty when the golden sequence
	// ran out of frames first.
	WantPath string

	// GotPath is the candidate frame at Index, or empty when the candidate
	// sequence ran out of frames first.
	GotPath string

	// Frame is the per-frame image difference. It is nil for a length
	// mismatch, where one sequence has a frame at Index and the other does not.
	Frame *Mismatch
}

// Error returns a human-readable reason for the sequence mismatch.
func (m *SequenceMismatch) Error() string {
	if m.Frame == nil {
		if m.GotPath == "" {
			return fmt.Sprintf("sequence length mismatch: golden has frame %d (%q) but candidate does not", m.Index, m.WantPath)
		}
		return fmt.Sprintf("sequence length mismatch: candidate has frame %d (%q) but golden does not", m.Index, m.GotPath)
	}
	return fmt.Sprintf("frame %d (%q vs %q): %v", m.Index, m.WantPath, m.GotPath, m.Frame)
}

// CompareSequences compares two ordered lists of PNG frame files pairwise and
// returns the first difference, or nil if the sequences are identical. Frames
// are compared up to the shorter length; if the lengths then differ, the extra
// leading frame is reported as a length mismatch. The returned error covers
// read and decode failures.
func CompareSequences(wantPaths, gotPaths []string) (*SequenceMismatch, error) {
	n := min(len(wantPaths), len(gotPaths))
	for i := range n {
		frame, err := ComparePNGFiles(wantPaths[i], gotPaths[i])
		if err != nil {
			return nil, fmt.Errorf("comparing frame %d: %w", i, err)
		}
		if frame != nil {
			return &SequenceMismatch{Index: i, WantPath: wantPaths[i], GotPath: gotPaths[i], Frame: frame}, nil
		}
	}

	if len(wantPaths) == len(gotPaths) {
		return nil, nil
	}

	mismatch := &SequenceMismatch{Index: n}
	if len(wantPaths) > n {
		mismatch.WantPath = wantPaths[n]
	} else {
		mismatch.GotPath = gotPaths[n]
	}
	return mismatch, nil
}

// PNGSequence returns the .png files directly under dir, sorted by name. That
// ordering matches a captured frame sequence whose filenames are zero-padded
// (frame_001.png, frame_002.png, ...), which is what [StepCapturer] produces.
func PNGSequence(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err // os.PathError already includes operation and filename
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
