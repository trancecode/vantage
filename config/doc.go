// Package config provides a layered configuration loader. A Loader merges, in
// increasing precedence, embedded default documents, game-registered default
// documents, a local TOML file, and section.key=value overrides into one or
// more registered target structs. It is generic and game-agnostic: the engine
// registers its settings and a game may register its own.
package config
