// Package render provides the graphics layer for the game.
//
// Camera handles world-to-screen coordinate transformation with zoom and pan.
// Sprite wraps directional animations loaded from sprite sheets, keyed by
// AnimationType, with automatic horizontal flip to generate left/right
// variants; Sprite.DrawAnimationScaled draws one at a per-call display scale,
// for views that need a different size without mutating the shared sprite.
// The anchor that a sprite is drawn and hit-tested against is per animation:
// SetZeroPosition sets one anchor across every animation on the sprite, which
// is what a uniform sheet wants, and Anchor reads the resolved value for a
// given AnimationType, resolving a mirrored animation to the animation it is
// drawn from. AnimationSpec describes one animation's frames, anchor and
// duration at load time; LoadSpriteAnimations builds a sprite from a map of
// them, and LoadSprite is the convenience for a uniform grid built on top of
// it. LoadSpriteAutoCropped crops each animation to its own content, repacks
// the frames into a smaller image before upload, and derives each animation's
// anchor from a sheet-wide one, so a sparse sheet costs only the video memory
// its content actually needs. RegisterAnimationName gives an AnimationType a
// display name for labels such as the sprite showcase's; AnimationName returns
// it, falling back to the type's generated String with the engine's Animation
// prefix trimmed. TextWriter renders text using loaded fonts. DrawNameplate and
// DrawFloatingBar anchor a label or a fraction bar a constant screen-pixel gap
// above a sprite's given animation, staying correctly placed across camera
// zoom. TileSize is engine configuration (default 16) for how many pixels one
// world tile occupies; a sprite may also declare SourceTileSize, the tile size
// its art was drawn for, which the engine corrects for at draw time so the
// same sprite scales correctly under any TileSize. DrawList collects drawable
// payloads and iterates them in painter's order (ascending layer, then
// ascending Y) for back-to-front 2D drawing.
//
// SpriteLibrary maps display names to sprites, with the package-level Sprites
// as the default library a game registers into at init time. SpriteFilter
// selects how sprites are resampled when drawn at anything other than their
// native size, defaulting to nearest so pixel art stays crisp.
//
// ScreenLogger draws the debug overlay, with the package-level Log as the
// shared instance; it lives here rather than in util so that util and the
// simulation packages above it stay free of any graphics dependency.
package render
