// Package scene defines the Scene interface and the Manager that drives the
// scene lifecycle: registration, per-frame update, and layer-ordered drawing,
// with per-scene visibility and exclusive focus. BaseScene is an embeddable
// default implementation, DialogScene is the engine's built-in modal dialog
// overlay, and SpriteShowcaseScene draws every sprite in a render.SpriteLibrary
// for visual inspection. Scenes are identified by a typed-string SceneName;
// each game defines its own names, and the engine reserves DialogSceneName and
// SpriteShowcaseSceneName.
package scene
