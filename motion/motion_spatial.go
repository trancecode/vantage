package motion

import "github.com/trancecode/vantage/geometry"

// Spatial holds an entity's position and facing in the game world. It is an ECS
// component: a plain data struct read and written through the world's component
// handles.
type Spatial struct {
	// Position is the current world coordinates of the entity in tile units.
	Position geometry.Vector2

	// Direction is the facing direction vector for orientation (unit vector).
	Direction geometry.Vector2
}
