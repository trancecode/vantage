package render

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// SpriteFilter is the texture filter used when a sprite is drawn at anything
// other than its native size, which happens whenever the camera zooms or a
// display scale is applied. Set by engine configuration; games choose it
// through the [render] filter setting.
//
// FilterNearest, the default, keeps pixel art crisp and blocky. FilterLinear
// smooths it, which suits high-resolution or hand-painted art but blurs
// pixel art.
var SpriteFilter = ebiten.FilterNearest

// Filter names accepted in settings and on the command line.
const (
	FilterNameNearest = "nearest"
	FilterNameLinear  = "linear"
)

// ParseFilter converts a settings filter name into an ebiten.Filter. It
// reports an error for any other name, listing the accepted ones.
func ParseFilter(name string) (ebiten.Filter, error) {
	switch name {
	case FilterNameNearest:
		return ebiten.FilterNearest, nil
	case FilterNameLinear:
		return ebiten.FilterLinear, nil
	default:
		return ebiten.FilterNearest, fmt.Errorf(
			"unknown filter %q, want %q or %q", name, FilterNameNearest, FilterNameLinear)
	}
}

// FilterName returns the settings name for a filter, so a loaded value can be
// written back out. It returns the nearest name for any unrecognized filter,
// matching SpriteFilter's default.
func FilterName(filter ebiten.Filter) string {
	if filter == ebiten.FilterLinear {
		return FilterNameLinear
	}
	return FilterNameNearest
}
