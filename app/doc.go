// Package app provides the engine's top-level App, which implements
// ebiten.Game so games do not have to. An App owns the window, the run loop,
// frame timing, the debug watchdog, screenshot capture, and exit handling, and
// embeds a scene.Manager. Games register scenes and, optionally, set OnUpdate
// for global per-frame logic.
package app
