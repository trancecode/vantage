// Package render provides the graphics layer for the game.
//
// Camera handles world-to-screen coordinate transformation with zoom and pan.
// Sprite wraps directional animations loaded from sprite sheets, with
// automatic horizontal flip to generate left/right variants. TextWriter
// renders text using loaded fonts. DrawNameplate and DrawFloatingBar anchor a
// label or a fraction bar a constant screen-pixel gap above a sprite, staying
// correctly placed across camera zoom. TileSize (16px) defines the base tile
// dimension used across the rendering pipeline.
package render
