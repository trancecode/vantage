// Package scene defines the Scene interface and the Manager that drives the
// scene lifecycle: registration, per-frame update, and layer-ordered drawing,
// with per-scene visibility and exclusive focus. BaseScene is an embeddable
// default implementation, and DialogScene is the engine's built-in modal
// dialog overlay. Scenes are identified by a typed-string SceneName; each game
// defines its own names, and the engine reserves DialogSceneName.
package scene
