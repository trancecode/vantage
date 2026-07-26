// Package render provides the graphics layer for the game.
//
// Camera handles world-to-screen coordinate transformation with zoom and pan.
// Sprite wraps directional animations loaded from sprite sheets, keyed by
// AnimationType, with automatic horizontal flip to generate left/right
// variants; Sprite.DrawAnimationScaled draws one at a per-call display scale,
// for views that need a different size without mutating the shared sprite.
// RegisterAnimationName gives an AnimationType a display name for labels such
// as the sprite showcase's; AnimationName returns it, falling back to the
// type's generated String with the engine's Animation prefix trimmed.
// TextWriter
// renders text using loaded fonts. DrawNameplate and DrawFloatingBar anchor a
// label or a fraction bar a constant screen-pixel gap above a sprite, staying
// correctly placed across camera zoom. TileSize is engine configuration
// (default 16) for how many pixels one world tile occupies; a sprite may also
// declare SourceTileSize, the tile size its art was drawn for, which the
// engine corrects for at draw time so the same sprite scales correctly under
// any TileSize. DrawList collects drawable payloads and iterates them in
// painter's order (ascending layer, then ascending Y) for back-to-front 2D
// drawing.
//
// SpriteLibrary maps display names to sprites, with the package-level Sprites
// as the default library a game registers into at init time. SpriteFilter
// selects how sprites are resampled when drawn at anything other than their
// native size, defaulting to nearest so pixel art stays crisp.
package render
