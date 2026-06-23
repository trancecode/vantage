# Style guide

## Errors

### General guidelines

* Never log and return; one or the other
* Never just return `err`, always provide additional context. If there is none to add, comment what is already there (e.g. `// os.PathError already includes operation and filename`)

### Formatting

Error messages should be phrased as `<context>: <reason>` where `<reason>` is generally the underlying error message.

The context should indicate the action being attempted.

Example:
```Go
if err := doSomething(); err != nil {
	return fmt.Errorf("doing something: %w", err)
}
```

### Providing context

* Context includes things like loop iterations and computed values the caller doesn't know or the reader might need
* Context includes what the code block is trying to do, not internals like function names
* Context must uniquely identify the code path when there could be multiple error returns
* Don't hesitate to use %T when dealing with unknown types
* Always use %q for strings we can't guarantee are clean, non-empty strings
* Include all context that the caller doesn't have, omit most context the caller does have
* Don't start with `failed to` or `error` except when you are logging

### Examples

* `loading sprite sheet 'player.png': decode png: unexpected EOF`

## Panic

Prefer `panic()` to `log.Fatalf()`. Use panic() for unrecoverable errors that should terminate the program immediately.

## Naming

* Use Java camel case convention for acronyms. e.g. use `FpsCounter` instead of `FPSCounter`.
* Use the idiomatic Go naming convention for everything else.

## Imports

* **Group imports:**
    * Standard library imports
    * Related third party imports
    * Local application/library specific imports
* **Import order within groups:** Sort alphabetically.

## Enumerations

When defining enumerations using `iota`, always start with a "None" or "Invalid" value as the zero value. This makes it explicit when enum fields are uninitialized or invalid.

**Preferred pattern:**
```go
type AnimationType int
const (
    AnimationTypeNone AnimationType = iota  // default/uninitialized value
    AnimationTypeIdle
    AnimationTypeMove
    AnimationTypeAttack
)
```

**Benefits:**
* **Explicit validation**: Can detect uninitialized enum fields (`if animationType == AnimationTypeNone`)
* **Debugging**: Clear indication when enum values haven't been set properly
* **Defensive programming**: Prevents subtle bugs from relying on implicit zero values
* **Code clarity**: Makes intent explicit rather than relying on implicit behavior

**Avoid:**
```go
type AnimationType int
const (
    AnimationTypeIdle AnimationType = iota  // implicit zero value
    AnimationTypeMove
    AnimationTypeAttack
)
```

This pattern should be applied to all new enumerations in the codebase.

## Comments

* **Write clear and concise comments:** Explain the "why" behind the code, not just the "what".
* **Comment sparingly:** Well-written code should be self-documenting where possible.
* **Use complete sentences:** Start comments with a capital letter and use proper punctuation.
* **Consider Context:** Place comments where they are most helpful for understanding the code, often above or to the right of the relevant code.
* **Be Concise:** Keep comments short and to the point, avoiding unnecessary verbosity.
* **Be Consistent:** Follow the established conventions within the project and the broader Go community.
* **Document Gotchas:** Explain any potential pitfalls or unusual behavior of the code.

## Documentation for packages, types and functions

* Document all exported types and functions.
* Start with the Name: Doc comments should generally begin with the name of the thing they're documenting (function, type, variable, etc.).
* Explain Purpose, Not Implementation: Focus on what the code is supposed to do, not the specific code logic.
* Keep the documentation concise and straightforward. The goal is to convey as much information as possible without using up too much of the context window. Explain the overall purpose, give details about corner cases.
* When describing a function: no need to describe each argument on its own. Just describe the function and how it uses the arguments.
* Use Proper Formatting: Utilize bullet points, code blocks (using tabs), and links where appropriate, leveraging Go's Markdown-like formatting.

### Struct field documentation

* **Document all exported struct fields:** Each exported field should have a comment explaining its purpose and usage.
* **Place field comments above the field:** Use the line above the field declaration for documentation, not inline comments.
* **Start with the field name:** Begin the comment with the field name followed by a description.
* **Be concise and specific:** Focus on the field's role and include units, ranges, or constraints where applicable.

**Preferred pattern:**
```go
// PositionComponent manages the spatial properties of entities in the game world.
type PositionComponent struct {
    // Position is the current world coordinates of the entity in tile units.
    Position geometry.Vector2

    // Layer is the rendering/collision layer for depth sorting.
    Layer Layer

    // Direction is the facing direction vector for orientation (unit vector).
    Direction geometry.Vector2
}
```

**Avoid:**
```go
type PositionComponent struct {
    Position geometry.Vector2  // world coordinates
    Layer Layer               // layer
    Direction geometry.Vector2 // direction
}
```

#### Specific examples

Example 1:
* Instead of: `// Increment the counter variable.`
* Prefer: `// Counter increments the value of a counter.`

Example 2:
* Instead of: `// This function calculates the sum of two numbers.`
* Prefer: `// Sum calculates the sum of two integers.`

## Specific modules and packages

* Use Ebiten v2: `github.com/hajimehoshi/ebiten/v2`
* Consume the entity-component-system layer directly from [`github.com/trancecode/ecs`](https://github.com/trancecode/ecs).

## File structure

* Within a package, logic is stored in files named after the system or component it relates to rather than the type the methods are from. For example, camera logic lives in `render_camera.go` and sprite logic in `render_sprite.go`, not in one file per type.

## Dos and don'ts

### Simple conditionals: avoid `else` blocks where applicable

Instead of:
```
if condition {
  // do something
} else {
  return
}
```

Prefer:
```
if !condition {
  return
}

// do something
```

### ECS: use the component handles natively

Packages that consume ECS (such as `motion` and `tilemap`) hold typed
`ecs.Accessor[T]` handles per component type. Read and mutate components through
those handles rather than wrapping them in per-type accessor methods.

1. **Read components with the handle's `Get`:**
```go
if component, ok := accessor.Get(id); ok {
    // use component
}
```

2. **Get-or-create with `GetOrAdd` / `GetOrAddFunc`** when a site genuinely needs
   fetch-or-lazily-create semantics (the component may or may not already exist):
```go
// returns a non-nil interior pointer
mc := movingComponents.GetOrAdd(id, motion.MovingComponent{})
```

3. **Create a brand-new component with a plain `Add`** when the component provably
   does not exist yet (e.g. on a freshly allocated entity):
```go
positionComponents.Add(id, motion.PositionComponent{})
```

Do not wrap these generic operations in accessor methods. If a helper only
restates a generic ECS operation (get / add / iterate / get-or-create / join),
use the handle directly; if a capability is missing, prefer contributing it
upstream to `trancecode/ecs` over re-introducing boilerplate. Methods that
coordinate ECS state with engine side-tables (spatial grid, tile occupancy) do
stay as domain methods on the type that owns those side-tables.

### Enumerations: validation and error handling

When working with enumerations, always validate that enum values are not in their "None" state before using them:

**Preferred:**
```go
func draw(animationType AnimationType) {
    if animationType == AnimationTypeNone {
        panic("animationType must be specified")
    }
    // proceed with drawing
}
```

**Also good for non-critical paths:**
```go
func process(animationType AnimationType) {
    if animationType == AnimationTypeNone {
        return // skip invalid values
    }
    // process
}
```

This ensures that enum fields are always explicitly set and prevents subtle bugs from uninitialized values.

### ECS: don't hold component references outside systems

Components are owned by the ECS storage layer, and the handles return interior
pointers that are valid only until the next structural change to that store.
Within a system, borrowing a component reference for the duration of a single
iteration is allowed. Outside of systems — in fields or containers that outlive
an iteration, such as priority queues, caches, or schedulers — reference entities
by `ecs.EntityId` and resolve the component through its handle at the point of
use. Never stash a component reference in a structure that outlives a single
system iteration.
