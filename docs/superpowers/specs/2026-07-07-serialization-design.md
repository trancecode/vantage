# Serialization / savegame design

How a game built on vantage persists and restores a running simulation. The
guiding principle: **the engine provides marshalable primitives and restore
hooks; the game owns the save-file format and orchestrates.** ecs is generic
over game-defined component types, so only the game knows how to encode them.
Keeping serialization game-driven avoids a codec registry or reflection-based
marshaling inside ecs.

## What a savegame must capture

For a lockstep-style deterministic sim, a save is everything needed to resume an
identical run:

* **The clock** — `sim.Driver.Now()` (a `util.Time`).
* **The event queue** — `sim.Driver.Queue().Snapshot()` (the pending events).
* **The RNG state** — so the random sequence continues (see `util.Rng`).
* **The entity-allocation counter** — so newly allocated entities after load do
  not collide with loaded ids.
* **The entities and their components** — per component type, the `(EntityId,
  component)` pairs.

## Engine-provided primitives (what vantage/ecs must expose)

Split into self-contained mechanical pieces (each a standalone brief) and the
design-heavier ecs world piece.

### Mechanical (independently shippable)

* **`ecs.EntityId` binary marshaling** — `MarshalBinary`/`UnmarshalBinary`, a
  fixed 8-byte big-endian of the internal counter. Foundation for every id in a
  save. (Brief: ecs-entityid-marshal.)
* **`sim.EventQueue` binary marshaling** — `MarshalBinary`/`UnmarshalBinary`
  over `Event{Time, Entity, Key}` (fixed-width: 8 + 8 + 8 bytes), using the
  EntityId marshaling for the entity field. Round-trips through `Snapshot`/
  `Restore`, so heap layout need not be preserved. (Brief: sim-eventqueue-marshal.)
* **`util.Rng`** — a seedable deterministic RNG wrapping `math/rand/v2.PCG`,
  with `MarshalBinary`/`UnmarshalBinary` (PCG already marshals to 20 bytes).
  Games use this instead of a bare `*rand.Rand` so the RNG state is saveable.
  (Brief: util-rng.)

### Design-heavier (ecs world restore)

ecs is a generic sparse-set ECS (`stores map[reflect.Type]componentStorage`,
`Accessor[C].All() iter.Seq2[EntityId, *C]`, private `nextID atomic.Uint64`,
`alive map[EntityId]struct{}`). To let a game save/restore the world without ecs
knowing the component types, ecs needs three additions:

* **Counter save/restore:** `func (w *World) EntityCounter() uint64` and
  `func (w *World) RestoreEntityCounter(n uint64)` — read the allocation counter
  on save, reseat it on load. The exact counter must round-trip (not merely
  "max live id"): ids are an event-queue tie-breaker, so a loaded run must
  allocate the same ids the saved run would have or determinism breaks.
* **Restore an entity with a fixed id:** `func (w *World) RestoreEntity(id
  EntityId) error` — mark an id alive without allocating a fresh one (allocation
  is what `NewEntity` does; restore must preserve the saved id). It errors on
  input a well-formed save cannot produce: the zero id, an id beyond the counter
  (restore the counter first), a duplicate id, or a call during an iteration.
  Components are then attached via the existing `Accessor.Add(id, component)`.
* **Enumeration already exists:** `Accessor[C].All()` yields `(EntityId, *C)` for
  save; the game holds one Accessor per component type and iterates each.

These are a focused ecs change but involve real API-design judgment (the restore
lifecycle, interaction with the deferred-command/iteration machinery), so they
belong in a strong-model session, not a mechanical Fable brief.

## Game-side orchestration (stays in the game)

Save (pseudocode; the game owns the byte format, e.g. length-prefixed sections):

```
write(driver.Now())                        // clock
write(driver.Queue().MarshalBinary())      // events
write(rng.MarshalBinary())                 // RNG
write(world.EntityCounter())               // allocation counter
write(sorted live entity ids)              // for RestoreEntity on load
for each component type T the game defines:
    for id, c := range accessorT.All():    // engine iteration
        write(id, encode(c))               // game encodes its own component T
```

Load:

```
w := ecs.NewWorld()
w.RestoreEntityCounter(readCounter)
for id in readEntityIds: w.RestoreEntity(id)
for each component type T: for each (id, blob): accessorT.Add(id, decode(blob))
driver := sim.NewDriver(handler)
driver.RestoreNow(readClock)
driver.RestoreQueue(sim.Restore(decode(readEvents)))
rng.UnmarshalBinary(readRng)
```

The engine never sees the game's component encoding; the game never reaches into
engine internals. Determinism holds because ids, clock, queue, and RNG are all
restored exactly.

## Non-goals / notes

* No versioned/upgradable save format in the engine — that is a game concern the
  game layers on top of these primitives.
* Component encoding (gob, hand-rolled, protobuf) is the game's choice; the
  engine only guarantees stable ids, clock, queue, and RNG round-trips.
* The four mechanical primitives can ship and be adopted independently; the ecs
  world-restore trio is the gating design piece for a full savegame.
