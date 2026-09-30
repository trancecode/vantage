// Package render provides the graphics layer for the game.
//
// Camera handles world-to-screen coordinate transformation with zoom and pan.
// Sprite wraps directional animations loaded from sprite sheets, keyed by
// AnimationType, with automatic horizontal flip to generate left/right
// variants; Sprite.DrawAnimationScaled draws one at a per-call display scale,
// for views that need a different size without mutating the shared sprite.
// An Animation's frames each carry an Offset: where the frame's image sits in
// the animation's frame space, the uncropped cell it was cut from. The anchor a
// sprite is drawn and hit-tested against is per animation and in frame space,
// so a frame cropped to its content is drawn with the anchor less its offset,
// and VisibleBounds reports frame space too. SetZeroPosition sets one anchor
// across every animation on the sprite, which is what a uniform sheet wants,
// and Anchor reads the resolved value for a given AnimationType, resolving a
// mirrored animation to the animation it is drawn from. AnimationSpec describes
// one animation's frames, anchor and duration at load time;
// LoadSpriteAnimations builds a sprite from a map of them, and LoadSprite is
// the convenience for a uniform grid built on top of it. LoadSpriteAutoCropped
// crops each frame to its own content, repacks the frames into a smaller image
// before upload, and records each frame's offset in its cell so the sheet-wide
// anchor serves every frame, so a sparse sheet costs only the video memory its
// content actually needs. RegisterAnimationName gives an AnimationType a
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
// Occludes reports whether a drawable occludes another based on their
// rectangles and base positions; Fader eases each drawable's opacity towards
// a faded value while it occludes and back to opaque once clear, managing the
// display state as drawables move and change occlusion, alongside DrawList.
//
// ScreenLogger draws the debug overlay, with the package-level Log as the
// shared instance; it lives here rather than in util so that util and the
// simulation packages above it stay free of any graphics dependency.
package render
