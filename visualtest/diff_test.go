package visualtest

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// solidImage returns a w×h image filled with c.
func solidImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

var (
	red   = color.RGBA{R: 255, A: 255}
	green = color.RGBA{G: 255, A: 255}
)

func TestCompareImagesIdentical(t *testing.T) {
	a := solidImage(4, 3, red)
	b := solidImage(4, 3, red)
	if m := CompareImages(a, b); m != nil {
		t.Fatalf("CompareImages of identical images = %v, want nil", m)
	}
}

func TestCompareImagesIdenticalAcrossColorModels(t *testing.T) {
	rgba := solidImage(2, 2, red)
	nrgba := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			nrgba.Set(x, y, red)
		}
	}
	if m := CompareImages(rgba, nrgba); m != nil {
		t.Fatalf("CompareImages across color models = %v, want nil", m)
	}
}

func TestCompareImagesSizeMismatch(t *testing.T) {
	a := solidImage(4, 3, red)
	b := solidImage(5, 3, red)
	m := CompareImages(a, b)
	if m == nil {
		t.Fatal("CompareImages of different sizes = nil, want mismatch")
	}
	if m.Kind != MismatchKindSize {
		t.Fatalf("Kind = %v, want MismatchKindSize", m.Kind)
	}
	if m.WantBounds.Dx() != 4 || m.GotBounds.Dx() != 5 {
		t.Fatalf("bounds = want %v got %v, expected widths 4 and 5", m.WantBounds, m.GotBounds)
	}
}

func TestCompareImagesPixelMismatch(t *testing.T) {
	a := solidImage(4, 3, red)
	b := solidImage(4, 3, red)
	b.Set(2, 1, green)

	m := CompareImages(a, b)
	if m == nil {
		t.Fatal("CompareImages with a differing pixel = nil, want mismatch")
	}
	if m.Kind != MismatchKindPixel {
		t.Fatalf("Kind = %v, want MismatchKindPixel", m.Kind)
	}
	if m.X != 2 || m.Y != 1 {
		t.Fatalf("coordinates = (%d, %d), want (2, 1)", m.X, m.Y)
	}
	if m.Want != red || m.Got != green {
		t.Fatalf("colors = want %v got %v, expected %v and %v", m.Want, m.Got, red, green)
	}
}

func TestCompareImagesReportsTopLeftmostPixel(t *testing.T) {
	a := solidImage(4, 4, red)
	b := solidImage(4, 4, red)
	// Two differences; the row-major scan must report the earlier one at (1, 0),
	// not the later one at (0, 2).
	b.Set(0, 2, green)
	b.Set(1, 0, green)

	m := CompareImages(a, b)
	if m == nil || m.X != 1 || m.Y != 0 {
		t.Fatalf("CompareImages = %v, want first mismatch at (1, 0)", m)
	}
}

func TestMismatchError(t *testing.T) {
	size := &Mismatch{Kind: MismatchKindSize, WantBounds: image.Rect(0, 0, 4, 3), GotBounds: image.Rect(0, 0, 5, 3)}
	if got := size.Error(); got != "size mismatch: want 4x3, got 5x3" {
		t.Errorf("size Error() = %q", got)
	}
	pixel := &Mismatch{Kind: MismatchKindPixel, X: 2, Y: 1, Want: red, Got: green}
	if got := pixel.Error(); got == "" {
		t.Error("pixel Error() is empty")
	}
}

// writePNG encodes img to a PNG file at path.
func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %q: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encoding %q: %v", path, err)
	}
}

func TestComparePNGFilesIdentical(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "want.png")
	got := filepath.Join(dir, "got.png")
	writePNG(t, want, solidImage(3, 3, red))
	writePNG(t, got, solidImage(3, 3, red))

	m, err := ComparePNGFiles(want, got)
	if err != nil {
		t.Fatalf("ComparePNGFiles: %v", err)
	}
	if m != nil {
		t.Fatalf("ComparePNGFiles of identical files = %v, want nil", m)
	}
}

func TestComparePNGFilesMissingFile(t *testing.T) {
	dir := t.TempDir()
	got := filepath.Join(dir, "got.png")
	writePNG(t, got, solidImage(3, 3, red))

	if _, err := ComparePNGFiles(filepath.Join(dir, "absent.png"), got); err == nil {
		t.Fatal("ComparePNGFiles with a missing golden = nil error, want error")
	}
}

func TestCompareSequencesIdentical(t *testing.T) {
	dir := t.TempDir()
	var want, got []string
	for i := range 3 {
		wp := filepath.Join(dir, "w"+string(rune('0'+i))+".png")
		gp := filepath.Join(dir, "g"+string(rune('0'+i))+".png")
		writePNG(t, wp, solidImage(2, 2, red))
		writePNG(t, gp, solidImage(2, 2, red))
		want = append(want, wp)
		got = append(got, gp)
	}
	m, err := CompareSequences(want, got)
	if err != nil {
		t.Fatalf("CompareSequences: %v", err)
	}
	if m != nil {
		t.Fatalf("CompareSequences of identical sequences = %v, want nil", m)
	}
}

func TestCompareSequencesFrameMismatch(t *testing.T) {
	dir := t.TempDir()
	w0 := filepath.Join(dir, "w0.png")
	w1 := filepath.Join(dir, "w1.png")
	g0 := filepath.Join(dir, "g0.png")
	g1 := filepath.Join(dir, "g1.png")
	writePNG(t, w0, solidImage(2, 2, red))
	writePNG(t, g0, solidImage(2, 2, red))
	writePNG(t, w1, solidImage(2, 2, red))
	writePNG(t, g1, solidImage(2, 2, green)) // second frame differs

	m, err := CompareSequences([]string{w0, w1}, []string{g0, g1})
	if err != nil {
		t.Fatalf("CompareSequences: %v", err)
	}
	if m == nil || m.Index != 1 {
		t.Fatalf("CompareSequences = %v, want mismatch at index 1", m)
	}
	if m.Frame == nil || m.Frame.Kind != MismatchKindPixel {
		t.Fatalf("frame mismatch = %v, want a pixel mismatch", m.Frame)
	}
}

func TestCompareSequencesLengthMismatch(t *testing.T) {
	dir := t.TempDir()
	w0 := filepath.Join(dir, "w0.png")
	w1 := filepath.Join(dir, "w1.png")
	g0 := filepath.Join(dir, "g0.png")
	writePNG(t, w0, solidImage(2, 2, red))
	writePNG(t, w1, solidImage(2, 2, red))
	writePNG(t, g0, solidImage(2, 2, red))

	m, err := CompareSequences([]string{w0, w1}, []string{g0})
	if err != nil {
		t.Fatalf("CompareSequences: %v", err)
	}
	if m == nil || m.Index != 1 {
		t.Fatalf("CompareSequences = %v, want length mismatch at index 1", m)
	}
	if m.Frame != nil {
		t.Fatalf("length mismatch Frame = %v, want nil", m.Frame)
	}
	if m.WantPath != w1 || m.GotPath != "" {
		t.Fatalf("length mismatch paths = want %q got %q, expected the extra golden frame", m.WantPath, m.GotPath)
	}
}

func TestPNGSequenceSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "frame_002.png"), solidImage(1, 1, red))
	writePNG(t, filepath.Join(dir, "frame_001.png"), solidImage(1, 1, red))
	writePNG(t, filepath.Join(dir, "frame_003.png"), solidImage(1, 1, red))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("writing non-png file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.png"), 0755); err != nil {
		t.Fatalf("creating directory: %v", err)
	}

	paths, err := PNGSequence(dir)
	if err != nil {
		t.Fatalf("PNGSequence: %v", err)
	}
	want := []string{
		filepath.Join(dir, "frame_001.png"),
		filepath.Join(dir, "frame_002.png"),
		filepath.Join(dir, "frame_003.png"),
	}
	if len(paths) != len(want) {
		t.Fatalf("PNGSequence = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("PNGSequence[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}
