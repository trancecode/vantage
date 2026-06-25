package ui

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/trancecode/vantage/asset"
)

const (
	dialogFontSize     = 16
	dialogTitleSize    = 20
	dialogPaddingX     = 24.0
	dialogPaddingY     = 20.0
	dialogOptionHeight = 36.0
	dialogMinWidth     = 300.0
	dialogSpacing      = 12.0
)

// DialogOption represents a selectable option in the dialog.
type DialogOption struct {
	// Label is the display text for this option.
	Label string

	// Detail is optional secondary text shown to the right (e.g., a price or cooldown).
	Detail string

	// OnSelect is called when this option is selected.
	OnSelect func()
}

// Dialog is a modal UI overlay with a title, optional context text, and selectable options.
type Dialog struct {
	// Title is displayed at the top of the dialog.
	Title string

	// Context is optional text displayed between the title and options.
	Context string

	// Options is the list of selectable options.
	Options []DialogOption

	// OnCancel is called when the dialog is dismissed with ESC.
	OnCancel func()

	// SelectedIndex is the currently highlighted option (for keyboard navigation).
	SelectedIndex int

	// screenWidth and screenHeight store the screen dimensions for centering.
	screenWidth  int
	screenHeight int

	// buttons holds the rendered button state for each option plus the cancel button.
	buttons []Button

	// cancelButton is the ESC/cancel button.
	cancelButton Button

	// layoutDirty tracks whether button positions need recalculation.
	layoutDirty bool

	// Pre-allocated images for rendering (avoid creating GPU resources every frame).
	overlayImage *ebiten.Image
	bgImage      *ebiten.Image
	borderPixel  *ebiten.Image
}

// NewDialog creates a dialog with the given title, options, and cancel callback.
func NewDialog(title string, options []DialogOption, onCancel func()) *Dialog {
	return &Dialog{
		Title:       title,
		Options:     options,
		OnCancel:    onCancel,
		layoutDirty: true,
	}
}

// SetScreenSize updates the screen dimensions used for centering the dialog.
func (d *Dialog) SetScreenSize(width, height int) {
	if d.screenWidth != width || d.screenHeight != height {
		d.screenWidth = width
		d.screenHeight = height
		d.layoutDirty = true
	}
}

// Update processes input for the dialog. Returns true if an option was selected or the dialog was cancelled.
func (d *Dialog) Update() bool {
	if d.layoutDirty {
		d.recalculateLayout()
	}

	// Mouse input
	mx, my := ebiten.CursorPosition()
	mouseX, mouseY := float64(mx), float64(my)
	mousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	// Update button states
	for i := range d.buttons {
		d.buttons[i].Update(mouseX, mouseY, mousePressed)
	}
	d.cancelButton.Update(mouseX, mouseY, mousePressed)

	// Mouse click on option
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i := range d.buttons {
			if d.buttons[i].containsPoint(mouseX, mouseY) {
				d.SelectedIndex = i
				if d.Options[i].OnSelect != nil {
					d.Options[i].OnSelect()
				}
				return true
			}
		}
		if d.cancelButton.containsPoint(mouseX, mouseY) {
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		}
	}

	// Keyboard: number keys 1-9 for options
	for i := range d.Options {
		if i >= 9 {
			break
		}
		key := ebiten.Key(int(ebiten.Key1) + i)
		if inpututil.IsKeyJustPressed(key) {
			d.SelectedIndex = i
			if d.Options[i].OnSelect != nil {
				d.Options[i].OnSelect()
			}
			return true
		}
	}

	// Keyboard: arrow keys for navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		d.SelectedIndex = (d.SelectedIndex + 1) % len(d.Options)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		d.SelectedIndex = (d.SelectedIndex - 1 + len(d.Options)) % len(d.Options)
	}

	// Enter to confirm selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if d.SelectedIndex >= 0 && d.SelectedIndex < len(d.Options) {
			if d.Options[d.SelectedIndex].OnSelect != nil {
				d.Options[d.SelectedIndex].OnSelect()
			}
			return true
		}
	}

	// ESC to cancel
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if d.OnCancel != nil {
			d.OnCancel()
		}
		return true
	}

	// Highlight the selected option's button as hovered if using keyboard
	for i := range d.buttons {
		if i == d.SelectedIndex && d.buttons[i].State == ButtonStateNormal {
			d.buttons[i].State = ButtonStateHovered
		}
	}

	return false
}

// Draw renders the dialog on the screen.
func (d *Dialog) Draw(screen *ebiten.Image) {
	if d.layoutDirty {
		d.recalculateLayout()
	}

	// Semi-transparent overlay covering the entire screen
	d.overlayImage.Fill(color.RGBA{0, 0, 0, 128})
	screen.DrawImage(d.overlayImage, nil)

	// Calculate dialog box dimensions
	dialogWidth, dialogHeight := d.dialogSize()
	dialogX := float64(d.screenWidth)/2 - dialogWidth/2
	dialogY := float64(d.screenHeight)/2 - dialogHeight/2

	// Draw dialog background
	d.bgImage.Fill(color.RGBA{30, 30, 45, 240})
	bgOp := &ebiten.DrawImageOptions{}
	bgOp.GeoM.Translate(dialogX, dialogY)
	screen.DrawImage(d.bgImage, bgOp)

	// Draw border (1px inset)
	d.drawBorder(screen, dialogX, dialogY, dialogWidth, dialogHeight, color.RGBA{100, 100, 140, 255})

	// Draw title
	titleFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogTitleSize)}
	titleMetrics := titleFace.Metrics()
	titleX := dialogX + dialogPaddingX
	titleY := dialogY + dialogPaddingY

	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Translate(titleX, titleY)
	titleOp.ColorScale.ScaleWithColor(color.RGBA{220, 220, 255, 255})
	text.Draw(screen, d.Title, titleFace, titleOp)

	// Draw context text if present
	if d.Context != "" {
		contextFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogFontSize)}
		contextY := titleY + titleMetrics.HAscent + titleMetrics.HDescent + dialogSpacing

		contextOp := &text.DrawOptions{}
		contextOp.GeoM.Translate(titleX, contextY)
		contextOp.ColorScale.ScaleWithColor(color.RGBA{180, 180, 200, 255})
		text.Draw(screen, d.Context, contextFace, contextOp)
	}

	// Draw option buttons
	for i := range d.buttons {
		d.buttons[i].Draw(screen, dialogFontSize)
	}

	// Draw cancel button
	d.cancelButton.Draw(screen, dialogFontSize)
}

func (d *Dialog) recalculateLayout() {
	dialogWidth, dialogHeight := d.dialogSize()
	dialogX := float64(d.screenWidth)/2 - dialogWidth/2
	dialogY := float64(d.screenHeight)/2 - dialogHeight/2

	// Pre-allocate rendering images
	d.overlayImage = ebiten.NewImage(d.screenWidth, d.screenHeight)
	d.bgImage = ebiten.NewImage(int(dialogWidth), int(dialogHeight))
	if d.borderPixel == nil {
		d.borderPixel = ebiten.NewImage(1, 1)
	}

	// Calculate the Y offset where options start
	optionsY := d.optionsStartY(dialogY)

	// Create buttons for options
	buttonWidth := dialogWidth - dialogPaddingX*2
	d.buttons = make([]Button, len(d.Options))
	for i, opt := range d.Options {
		label := opt.Label
		if opt.Detail != "" {
			label = fmt.Sprintf("%s  %s", opt.Label, opt.Detail)
		}
		d.buttons[i] = Button{
			Label:         label,
			ShortcutLabel: strconv.Itoa(i + 1),
			X:             dialogX + dialogPaddingX,
			Y:             optionsY + float64(i)*dialogOptionHeight,
			Width:         buttonWidth,
			Height:        dialogOptionHeight - 4, // Small gap between buttons
			State:         ButtonStateNormal,
			bgImage:       ebiten.NewImage(int(buttonWidth), int(dialogOptionHeight-4)),
		}
	}

	// Cancel button at the bottom
	cancelY := optionsY + float64(len(d.Options))*dialogOptionHeight + dialogSpacing
	d.cancelButton = Button{
		Label:         "Cancel",
		ShortcutLabel: "ESC",
		X:             dialogX + dialogPaddingX,
		Y:             cancelY,
		Width:         buttonWidth,
		Height:        dialogOptionHeight - 4,
		State:         ButtonStateNormal,
		bgImage:       ebiten.NewImage(int(buttonWidth), int(dialogOptionHeight-4)),
	}

	d.layoutDirty = false
}

func (d *Dialog) dialogSize() (float64, float64) {
	// Measure title width
	titleFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogTitleSize)}
	titleWidth := text.Advance(d.Title, titleFace)

	// Measure option widths
	optionFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogFontSize)}
	maxOptionWidth := 0.0
	for i, opt := range d.Options {
		label := fmt.Sprintf("[%d] %s", i+1, opt.Label)
		if opt.Detail != "" {
			label += "  " + opt.Detail
		}
		w := text.Advance(label, optionFace)
		if w > maxOptionWidth {
			maxOptionWidth = w
		}
	}
	// Cancel button width
	cancelWidth := text.Advance("[ESC] Cancel", optionFace)
	if cancelWidth > maxOptionWidth {
		maxOptionWidth = cancelWidth
	}

	// Width: max of title, options, and minimum
	contentWidth := titleWidth
	if maxOptionWidth+24 > contentWidth { // 24 for shortcut padding
		contentWidth = maxOptionWidth + 24
	}
	width := contentWidth + dialogPaddingX*2
	if width < dialogMinWidth {
		width = dialogMinWidth
	}

	// Height: content above options + options + cancel + padding
	height := d.contentHeightAboveOptions()
	height += float64(len(d.Options)) * dialogOptionHeight // Option buttons
	height += dialogSpacing                                // Gap before cancel
	height += dialogOptionHeight                           // Cancel button
	height += dialogPaddingY                               // Bottom padding

	return width, height
}

func (d *Dialog) optionsStartY(dialogY float64) float64 {
	return dialogY + d.contentHeightAboveOptions()
}

// contentHeightAboveOptions returns the vertical space occupied by the top
// padding, the title, and the optional context (each followed by spacing) —
// i.e. the offset from the dialog's top edge to where the option buttons begin.
func (d *Dialog) contentHeightAboveOptions() float64 {
	titleFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogTitleSize)}
	titleMetrics := titleFace.Metrics()

	h := dialogPaddingY
	h += titleMetrics.HAscent + titleMetrics.HDescent
	h += dialogSpacing

	if d.Context != "" {
		contextFace := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(dialogFontSize)}
		contextMetrics := contextFace.Metrics()
		h += contextMetrics.HAscent + contextMetrics.HDescent
		h += dialogSpacing
	}

	return h
}

func (d *Dialog) drawBorder(screen *ebiten.Image, x, y, width, height float64, c color.RGBA) {
	d.borderPixel.Fill(c)

	// Top
	topOp := &ebiten.DrawImageOptions{}
	topOp.GeoM.Scale(width, 1)
	topOp.GeoM.Translate(x, y)
	screen.DrawImage(d.borderPixel, topOp)

	// Bottom
	bottomOp := &ebiten.DrawImageOptions{}
	bottomOp.GeoM.Scale(width, 1)
	bottomOp.GeoM.Translate(x, y+height-1)
	screen.DrawImage(d.borderPixel, bottomOp)

	// Left
	leftOp := &ebiten.DrawImageOptions{}
	leftOp.GeoM.Scale(1, height)
	leftOp.GeoM.Translate(x, y)
	screen.DrawImage(d.borderPixel, leftOp)

	// Right
	rightOp := &ebiten.DrawImageOptions{}
	rightOp.GeoM.Scale(1, height)
	rightOp.GeoM.Translate(x+width-1, y)
	screen.DrawImage(d.borderPixel, rightOp)
}
