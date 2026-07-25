// Package render provides the graphics layer for the game.
//
// Camera handles world-to-screen coordinate transformation with zoom and pan.
// Sprite wraps directional animations loaded from sprite sheets, with
// automatic horizontal flip to generate left/right variants;
// Sprite.DrawAnimationScaled draws one at a per-call display scale, for views
// that need a different size without mutating the shared sprite. TextWriter
// renders text using loaded fonts. DrawNameplate and DrawFloatingBar anchor a
// label or a fraction bar a constant screen-pixel gap above a sprite, staying
// correctly placed across camera zoom. TileSize (16px) defines the base tile
// dimension used across the rendering pipeline. DrawList collects drawable
// payloads and iterates them in painter's order (ascending layer, then
// ascending Y) for back-to-front 2D drawing.
//
// SpriteLibrary maps display names to sprites, with the package-level Sprites
// as the default library a game registers into at init time.
package render
