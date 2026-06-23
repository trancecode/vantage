package render

// SpriteType represents different types of sprites in the game.
//
//go:generate stringer -type=SpriteType -output=render_spritetype_string.go
type SpriteType int

const (
	SpriteTypeUnknown SpriteType = 0
	SpriteTypeActor   SpriteType = 1
	SpriteTypeTerrain SpriteType = 2
)
