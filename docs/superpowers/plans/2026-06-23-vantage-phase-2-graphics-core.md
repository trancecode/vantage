# Vantage extraction — Phase 2: graphics core

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the graphics core (`render`, `ui`) from nrg into vantage, with the game-specific sprite catalog removed, fonts injected from a new engine `asset` package that embeds two OFL fonts, and the camera split into a pure transform plus a pluggable controller.

**Architecture:** `render` and `ui` are game-agnostic graphics mechanics. The only couplings to nrg content are the default font (`data.FontDefault`) and the sprite catalog (`render_sprite_data.go`). This phase introduces a low-level `asset` package (embedded fonts) that both `render` and `ui` depend on, removes the catalog so games build their own, and separates `Camera` (transform math) from `CameraController` (the default pan/zoom input scheme).

**Tech Stack:** Go 1.26.4, Ebitengine v2 (`text/v2` for fonts), `github.com/spf13/pflag` (carried over with `render`), Google Sans Flex + Google Sans Code (OFL).

## Global Constraints

* Module path: `github.com/trancecode/vantage`. Rewrite every moved file's `github.com/herve-quiroz/nrg/<pkg>` import to `github.com/trancecode/vantage/<pkg>`.
* Go version: `1.26.4` (canonical in `go.mod`). Do not change it.
* Set `GOMODCACHE=/tmp/go-mod-cache` before any Go command.
* Ebiten-dependent tests run under a virtual display: `xvfb-run -a go test ...`. The `asset` package test does not need a display (font parsing only) but falls back to xvfb if a display error appears.
* Licensing: MIT for code; bundled fonts are OFL and MUST ship their `OFL.txt` beside the `.ttf` under `asset/font/<name>/`.
* `go:embed` patterns cannot contain `[` or `]` (glob metacharacters). Vendored font files MUST be renamed to bracket-free names (`GoogleSansCode.ttf`, `GoogleSansFlex.ttf`).
* Moved mechanics files change ONLY by import-path rewrite and the explicit edits named in each task. Do not refactor or reformat unrelated code.
* Commit author: name `Claude Code`, email `herve.quiroz+claude@gmail.com`. No `Co-Authored-By` line.
* Work directly on `main`; commit and push per task.
* Source of truth for moved packages: the nrg working copy at `/home/hqz/src/nrg`.
* Carry-forward from Phase 1: do NOT modify `util`/`geometry`; keep them byte-identical to nrg.

## Verified font sources (used in Task 1)

* **Google Sans Code** (monospace): release zip `v7.001` at `https://github.com/googlefonts/googlesans-code/releases/download/v7.001/GoogleSansCode-v7.001.zip`, containing `GoogleSansCode[MONO,wght].ttf` (268 KB). OFL at `https://raw.githubusercontent.com/googlefonts/googlesans-code/main/OFL.txt`. Confirmed valid TrueType and loadable by `text.NewGoTextFaceSource`.
* **Google Sans Flex** (proportional): `https://raw.githubusercontent.com/LineageOS/android_external_google-fonts_google-sans-flex/lineage-23.2/GoogleSansFlex-Regular.ttf` (4 MB). OFL license at `.../lineage-23.2/LICENSE`. Confirmed valid TrueType, loadable, and the license text is SIL OFL 1.1. (Flex has no official googlefonts repo; this reputable mirror is the GitHub-accessible TTF source, and OFL permits redistribution.)

## File structure (Phase 2)

* `asset/asset_font.go` — embeds the two fonts; `LoadFont`/`MustLoadFont`; `DefaultProportionalFont`, `DefaultMonospaceFont`.
* `asset/doc.go` — package doc.
* `asset/asset_font_test.go` — asserts both default fonts loaded.
* `asset/font/google-sans-code/GoogleSansCode.ttf` + `OFL.txt`.
* `asset/font/google-sans-flex/GoogleSansFlex.ttf` + `OFL.txt`.
* `render/*.go` — moved from nrg EXCEPT `render_sprite_data.go` and `render_sprite_string.go` (the `SpriteID` catalog and its stringer). `render_text.go` font default sourced from `asset`. `render_camera.go` reduced to transform; new `render_cameracontroller.go`.
* `render/render_camera_test.go` — new transform + controller tests.
* `ui/*.go` — moved from nrg; `data.FontDefault` replaced with `asset.DefaultProportionalFont`.

---

## Task 1: Engine `asset` package with embedded OFL fonts

**Files:**
- Create: `~/src/vantage/asset/font/google-sans-code/GoogleSansCode.ttf`, `OFL.txt`
- Create: `~/src/vantage/asset/font/google-sans-flex/GoogleSansFlex.ttf`, `OFL.txt`
- Create: `~/src/vantage/asset/asset_font.go`, `~/src/vantage/asset/doc.go`
- Test: `~/src/vantage/asset/asset_font_test.go`

**Interfaces:**
- Produces: `github.com/trancecode/vantage/asset` exporting `DefaultProportionalFont *text.GoTextFaceSource` (Google Sans Flex), `DefaultMonospaceFont *text.GoTextFaceSource` (Google Sans Code), `LoadFont([]byte) (*text.GoTextFaceSource, error)`, `MustLoadFont([]byte) *text.GoTextFaceSource`. Consumed by `render` (Task 2) and `ui` (Task 4).

- [ ] **Step 1: Vendor the fonts (download, rename bracket-free, place OFL beside each)**

Run:
```bash
cd ~/src/vantage
mkdir -p asset/font/google-sans-code asset/font/google-sans-flex

# Google Sans Code (monospace) from release zip
curl -fsSL -o /tmp/gsc.zip "https://github.com/googlefonts/googlesans-code/releases/download/v7.001/GoogleSansCode-v7.001.zip"
unzip -o -q /tmp/gsc.zip -d /tmp/gsc
cp "/tmp/gsc/GoogleSansCode[MONO,wght].ttf" asset/font/google-sans-code/GoogleSansCode.ttf
curl -fsSL -o asset/font/google-sans-code/OFL.txt "https://raw.githubusercontent.com/googlefonts/googlesans-code/main/OFL.txt"

# Google Sans Flex (proportional) from LineageOS mirror
curl -fsSL -o asset/font/google-sans-flex/GoogleSansFlex.ttf "https://raw.githubusercontent.com/LineageOS/android_external_google-fonts_google-sans-flex/lineage-23.2/GoogleSansFlex-Regular.ttf"
curl -fsSL -o asset/font/google-sans-flex/OFL.txt "https://raw.githubusercontent.com/LineageOS/android_external_google-fonts_google-sans-flex/lineage-23.2/LICENSE"
```

- [ ] **Step 2: Verify the vendored files**

Run:
```bash
cd ~/src/vantage
file asset/font/google-sans-code/GoogleSansCode.ttf asset/font/google-sans-flex/GoogleSansFlex.ttf
grep -ci "open font license" asset/font/google-sans-code/OFL.txt asset/font/google-sans-flex/OFL.txt
```
Expected: both `.ttf` report `TrueType Font data`; both OFL files report a non-zero count. If a download produced an HTML error page or a zero-byte file, stop and report — do not proceed with a bad font.

- [ ] **Step 3: Write `asset/asset_font.go`**

```go
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
```

- [ ] **Step 4: Write `asset/doc.go`**

```go
// Package asset provides engine-bundled assets. It embeds the engine's two
// default fonts — Google Sans Flex (proportional) and Google Sans Code
// (monospace), both under the SIL Open Font License — and exposes helpers to
// load additional fonts supplied by a game.
package asset
```

- [ ] **Step 5: Write the test `asset/asset_font_test.go`**

```go
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
```

The default fonts are loaded with `MustLoadFont` at package init, so a parse failure panics before the test runs; reaching the test with non-nil values proves both fonts parsed.

- [ ] **Step 6: Resolve deps and run the test**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go test ./asset/...
```
Expected: PASS (`ok github.com/trancecode/vantage/asset`). If it fails with a display/GLFW error, re-run as `xvfb-run -a go test ./asset/...`.

- [ ] **Step 7: Commit**

```bash
cd ~/src/vantage
git add asset/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add asset package with embedded OFL default fonts

Embeds Google Sans Flex (proportional) and Google Sans Code (monospace),
both under the SIL Open Font License, with each font's OFL.txt alongside it."
git push origin main
```

---

## Task 2: Extract the `render` package (mechanics only, fonts from `asset`)

Copy `render` from nrg EXCLUDING the catalog files, rewrite the `geometry` import path, and replace the `data.FontDefault` reference with `asset.DefaultProportionalFont`. The camera split is Task 3, not here.

**Files:**
- Create (copied from `/home/hqz/src/nrg/render/`): `doc.go`, `render_animation.go`, `render_animation_string.go`, `render_animation_test.go`, `render_camera.go`, `render_draw.go`, `render_sprite.go`, `render_spritetype.go`, `render_spritetype_string.go`, `render_text.go`
- DO NOT copy: `render_sprite_data.go`, `render_sprite_string.go` (the `SpriteID` catalog and its stringer — game content; leaves the engine)
- Modify after copy: `render_text.go` (font source)

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/geometry`, `github.com/trancecode/vantage/asset`.
- Produces: `github.com/trancecode/vantage/render` exporting `Camera`, `Sprite`, `Animation`, `AnimationType`, `SpriteType`, `TextWriter`, `Drawable`, `LoadSprite`, `MustLoadSprite`, `NewTextWriter`, `TextDefault`, the `TileSize` const, etc. (Catalog symbols `SpriteID`/`Sprites` are intentionally absent.)

- [ ] **Step 1: Copy render, excluding the two catalog files**

Run:
```bash
cd ~/src/vantage && mkdir -p render
for f in doc.go render_animation.go render_animation_string.go render_animation_test.go render_camera.go render_draw.go render_sprite.go render_spritetype.go render_spritetype_string.go render_text.go; do
  cp "/home/hqz/src/nrg/render/$f" render/
done
ls render/
```
Expected: the 10 files above, and NOT `render_sprite_data.go` or `render_sprite_string.go`.

- [ ] **Step 2: Rewrite the geometry import path across render**

Run:
```bash
cd ~/src/vantage
sed -i 's#github.com/herve-quiroz/nrg/geometry#github.com/trancecode/vantage/geometry#g' render/*.go
```

- [ ] **Step 3: Replace the font default in `render_text.go`**

In `render/render_text.go`, change the import block: remove the line
`"github.com/herve-quiroz/nrg/data"` and add `"github.com/trancecode/vantage/asset"` (keep imports grouped: stdlib, then third-party ebiten, then `github.com/trancecode/vantage/...` alphabetical). Then change the field initializer in `NewTextWriter` from:
```go
		Font:              data.FontDefault,
```
to:
```go
		Font:              asset.DefaultProportionalFont,
```

- [ ] **Step 4: Confirm no nrg imports remain in render**

Run:
```bash
cd ~/src/vantage && grep -rn 'herve-quiroz/nrg' render/ && echo "STILL HAS NRG IMPORTS" || echo "render decoupled from nrg"
```
Expected: `render decoupled from nrg`.

- [ ] **Step 5: Resolve deps, build, vet, and run render tests under xvfb**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./render/... && xvfb-run -a go test ./render/...
```
Expected: `go.mod` gains `github.com/spf13/pflag` (used by `render_sprite.go`'s `use_placeholder_sprite_images` flag); vet clean; tests PASS (`ok github.com/trancecode/vantage/render`).

- [ ] **Step 6: Commit**

```bash
cd ~/src/vantage
git add render/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract render package from nrg (catalog removed, fonts from asset)

Moves the rendering mechanics (camera, sprite, animation, text, drawable)
into vantage. The nrg-specific sprite catalog (render_sprite_data.go and its
SpriteID stringer) is intentionally left behind; games build their own
catalog via LoadSprite. TextWriter's default font now comes from the engine
asset package instead of nrg's data package."
git push origin main
```

---

## Task 3: Split `Camera` (transform) from `CameraController` (input)

Reduce `Camera` to pure transform math and move the pan/zoom input scheme into a new pluggable `CameraController`. Behavior of the default scheme is preserved exactly (WASD pan, Q/E and wheel zoom, middle-mouse drag).

**Files:**
- Modify: `render/render_camera.go`
- Create: `render/render_cameracontroller.go`
- Test: `render/render_camera_test.go` (new)

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/geometry`.
- Produces: `Camera` (no input methods; gains `AddZoom(delta float64)`, and `SetZoom` now clamps). New `CameraController` with exported fields `Camera *Camera`, `MoveSpeed float64`, `ZoomSpeed float64`; `NewCameraController(*Camera) *CameraController`; methods `HandleInput()` and `CursorWorldPosition() geometry.Vector2`.

- [ ] **Step 1: Reduce `Camera` to a transform in `render/render_camera.go`**

Make these edits to `render/render_camera.go`:

1. In the `Camera` struct, DELETE the fields `moveSpeed`, `zoomSpeed`, `lastMouseX, lastMouseY`, and `isMMBPressed`. Keep `pos`, `zoom`, `screenMultiplier`, `minZoom`, `maxZoom`, `screenWidth, screenHeight`.

2. In `NewCamera`, DELETE the `moveSpeed: 5,` and `zoomSpeed: 0.1,` initializers. The returned struct keeps `pos`, `zoom`, `screenMultiplier`, `minZoom`, `maxZoom`, `screenWidth`, `screenHeight`.

3. In `NewScreenCamera`, DELETE the `moveSpeed: 0,` and `zoomSpeed: 0,` initializers.

4. DELETE the methods `MoveSpeed()`, `ZoomSpeed()`, `HandleInput()`, and `CursorPosition()` entirely.

5. Change `SetZoom` to clamp:
```go
// SetZoom sets the camera's zoom level, clamped to the camera's limits.
func (c *Camera) SetZoom(zoom float64) {
	c.zoom = zoom
	c.clampZoom()
}
```

6. Add an `AddZoom` method (place it right after `SetZoom`):
```go
// AddZoom adjusts the zoom level by delta, clamped to the camera's limits.
func (c *Camera) AddZoom(delta float64) {
	c.zoom += delta
	c.clampZoom()
}
```

Keep everything else (`Position`, `SetPosition`, `SetZeroAsCenter`, `SetZeroAsTopLeft`, `Zoom`, `Move`, `clampZoom`, `ScreenWidth`, `ScreenHeight`, `CameraDebugInfo`, `DrawImageOptions`, `Adjust`, `ScreenToWorld`, `WorldToScreen`, the `TileSize` const) unchanged. After deleting `HandleInput`/`CursorPosition`, the `ebiten` import is still needed (used by `DrawImageOptions`/`Adjust`); leave it.

- [ ] **Step 2: Create `render/render_cameracontroller.go`**

```go
package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/geometry"
)

// CameraController drives a Camera from user input. It implements the engine's
// default pan/zoom control scheme: WASD keyboard panning, Q/E and mouse-wheel
// zoom, and middle-mouse-button drag panning. Games wanting a different scheme
// can drive the Camera directly instead of attaching a controller.
type CameraController struct {
	// Camera is the camera this controller drives.
	Camera *Camera
	// MoveSpeed is the pan speed in world units per frame, before zoom scaling.
	MoveSpeed float64
	// ZoomSpeed is the zoom increment applied per input step.
	ZoomSpeed float64

	lastMouseX, lastMouseY int
	isMMBPressed           bool
}

// NewCameraController returns a controller driving the given camera with the
// engine's default pan and zoom speeds.
func NewCameraController(camera *Camera) *CameraController {
	return &CameraController{
		Camera:    camera,
		MoveSpeed: 5,
		ZoomSpeed: 0.1,
	}
}

// HandleInput reads input for the current frame and pans/zooms the camera.
func (cc *CameraController) HandleInput() {
	c := cc.Camera
	moveSpeed := cc.MoveSpeed * c.Zoom()
	delta := geometry.NewVector2(0, 0)

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		delta = geometry.NewVector2(delta.X(), delta.Y()+moveSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		delta = geometry.NewVector2(delta.X(), delta.Y()-moveSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		delta = geometry.NewVector2(delta.X()-moveSpeed, delta.Y())
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		delta = geometry.NewVector2(delta.X()+moveSpeed, delta.Y())
	}
	c.Move(delta)

	// Middle mouse button drag for panning.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		currentX, currentY := ebiten.CursorPosition()
		if cc.isMMBPressed {
			deltaX := float64(currentX - cc.lastMouseX)
			deltaY := float64(currentY - cc.lastMouseY)
			c.SetPosition(geometry.NewVector2(c.Position().X()+deltaX, c.Position().Y()+deltaY))
		}
		cc.lastMouseX = currentX
		cc.lastMouseY = currentY
		cc.isMMBPressed = true
	} else {
		cc.isMMBPressed = false
	}

	// Scroll wheel zoom.
	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		c.AddZoom(wheelY * cc.ZoomSpeed)
	}

	// Q/E keyboard zoom.
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		c.AddZoom(-cc.ZoomSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		c.AddZoom(cc.ZoomSpeed)
	}
}

// CursorWorldPosition returns the OS cursor position converted to world
// coordinates through the controller's camera.
func (cc *CameraController) CursorWorldPosition() geometry.Vector2 {
	return cc.Camera.ScreenToWorld(geometry.NewVector2(ebiten.CursorPosition()))
}
```

Note: `SetPosition` calls SetPosition vs the original direct `c.pos =` are equivalent; `AddZoom` performs the clamp the original did at the end of `HandleInput`, and clamping is idempotent, so the scheme's net behavior is unchanged.

- [ ] **Step 3: Write `render/render_camera_test.go`**

```go
package render

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

func TestCameraWorldScreenRoundTrip(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZeroAsCenter()
	world := geometry.NewVector2(3.5, -2.0)
	got := c.ScreenToWorld(c.WorldToScreen(world))
	const eps = 1e-9
	if diff := got.X() - world.X(); diff > eps || diff < -eps {
		t.Fatalf("round-trip X = %v, want %v", got.X(), world.X())
	}
	if diff := got.Y() - world.Y(); diff > eps || diff < -eps {
		t.Fatalf("round-trip Y = %v, want %v", got.Y(), world.Y())
	}
}

func TestSetZoomClampsToMax(t *testing.T) {
	over := NewCamera(800, 600)
	over.SetZoom(1000) // far above maxZoom
	atMax := NewCamera(800, 600)
	atMax.SetZoom(5) // maxZoom
	if over.Zoom() != atMax.Zoom() {
		t.Fatalf("SetZoom not clamped: over=%v atMax=%v", over.Zoom(), atMax.Zoom())
	}
}

func TestAddZoomClampsToMin(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZoom(1.0)
	c.AddZoom(-1000) // far below minZoom
	atMin := NewCamera(800, 600)
	atMin.SetZoom(0.2) // minZoom
	if c.Zoom() != atMin.Zoom() {
		t.Fatalf("AddZoom not clamped to min: got=%v atMin=%v", c.Zoom(), atMin.Zoom())
	}
}

func TestNewCameraControllerDefaults(t *testing.T) {
	cc := NewCameraController(NewCamera(800, 600))
	if cc.Camera == nil {
		t.Fatal("controller camera is nil")
	}
	if cc.MoveSpeed != 5 || cc.ZoomSpeed != 0.1 {
		t.Fatalf("unexpected defaults: MoveSpeed=%v ZoomSpeed=%v", cc.MoveSpeed, cc.ZoomSpeed)
	}
}
```

- [ ] **Step 4: Build, vet, and run render tests under xvfb**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./render/... && xvfb-run -a go test ./render/...
```
Expected: vet clean; tests PASS, including the four new camera tests and the pre-existing animation test.

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add render/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Split Camera transform from CameraController input

Camera is now pure transform math (no Ebiten input); it gains AddZoom and a
clamping SetZoom. The default pan/zoom scheme (WASD, Q/E, wheel, middle-mouse
drag) moves into a new pluggable CameraController. Games can use the default
controller or drive the Camera directly."
git push origin main
```

---

## Task 4: Extract the `ui` package (fonts from `asset`)

Copy `ui` from nrg and replace every `data.FontDefault` reference with `asset.DefaultProportionalFont`.

**Files:**
- Create (copied from `/home/hqz/src/nrg/ui/`): `doc.go`, `ui_button.go`, `ui_button_test.go`, `ui_dialog.go`
- Modify after copy: `ui_button.go`, `ui_dialog.go` (and `ui_button_test.go` if it references the font)

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/asset` (and `github.com/trancecode/vantage/geometry` only if the source files import it).
- Produces: `github.com/trancecode/vantage/ui` exporting `Button`, `Dialog` with the same surface as nrg's `ui`.

- [ ] **Step 1: Copy the package**

Run:
```bash
cd ~/src/vantage && mkdir -p ui
cp /home/hqz/src/nrg/ui/doc.go /home/hqz/src/nrg/ui/ui_button.go /home/hqz/src/nrg/ui/ui_button_test.go /home/hqz/src/nrg/ui/ui_dialog.go ui/
```

- [ ] **Step 2: Rewrite imports — data→asset, and geometry path if present**

Run:
```bash
cd ~/src/vantage
sed -i 's#"github.com/herve-quiroz/nrg/data"#"github.com/trancecode/vantage/asset"#g' ui/*.go
sed -i 's#github.com/herve-quiroz/nrg/geometry#github.com/trancecode/vantage/geometry#g' ui/*.go
sed -i 's#data\.FontDefault#asset.DefaultProportionalFont#g' ui/*.go
grep -rn 'herve-quiroz/nrg\|data\.FontDefault' ui/ && echo "STILL COUPLED" || echo "ui decoupled from nrg"
```
Expected: `ui decoupled from nrg`. (If any file imported `data` for something other than `FontDefault`, the `grep` will reveal a leftover `data.` reference — stop and report, because this phase only handles the font coupling.)

- [ ] **Step 3: Build, vet, and run ui tests under xvfb**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./ui/... && xvfb-run -a go test ./ui/...
```
Expected: vet clean; tests PASS (`ok github.com/trancecode/vantage/ui`). If `ui_button_test.go` fails to compile because it referenced `data.FontDefault`, the Step-2 sed already rewrote it to `asset.DefaultProportionalFont`; if it referenced some other `data` symbol, stop and report.

- [ ] **Step 4: Full-module sanity build and test**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go build ./... && xvfb-run -a go test ./...
```
Expected: all packages (`asset`, `geometry`, `render`, `ui`, `util`) build and test PASS.

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add ui/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract ui package from nrg (fonts from asset)

Moves Button and Dialog into vantage. Their default font now comes from the
engine asset package instead of nrg's data package, removing ui's dependency
on game content."
git push origin main
```

---

## Self-review (Phase 2)

* **Spec coverage:** Implements the Phase-2 roadmap items from the design spec — `render` and `ui` extraction, sprite-catalog removal (games build their own via `LoadSprite`), `AnimationType`/`SpriteType` retained, fonts injected with Google Sans Flex (proportional) + Google Sans Code (monospace) embedded under OFL, and the `Camera`/`CameraController` split. Config-driven camera bindings are deferred to Phase 4 (the controller exposes `MoveSpeed`/`ZoomSpeed` fields and default key handling now; config wiring comes later).
* **Placeholders:** none. Font sources are verified and exact. The one conditional (whether `ui_button_test.go` references the font) is handled by the Step-2 sed plus an explicit stop-and-report guard.
* **Type consistency:** `asset.DefaultProportionalFont`/`DefaultMonospaceFont` defined in Task 1 are the exact names used in Tasks 2 and 4. `Camera.AddZoom`/clamping `SetZoom` defined in Task 3 Step 1 are the exact methods the Task 3 controller and tests call. `CameraController` field/method names are consistent between the source file and the test.
* **Deferred/known:** `render_sprite.go` keeps the package-level `use_placeholder_sprite_images` pflag — intentionally left for the Phase 4 config migration; it adds `spf13/pflag` to the module. The `TileSize` const stays in `render` (tile-unit coupling in the camera is pre-existing and out of scope).
