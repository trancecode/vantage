# Continuous movement building blocks: design

**Status:** Approved in conversation on 2026-09-30.

## Purpose

Let a game's characters stand and walk anywhere on the ground, in straight legs, instead of
hopping from one tile centre to the next. The request comes from nrg, stage 2 of its
[continuous movement design](https://github.com/herve-quiroz/nrg/blob/main/docs/superpowers/specs/2026-09-30-continuous-movement-design.md)
(commit `5b1db1a`), which follows the model nrg-rs built in its
[milestone 3](https://github.com/herve-quiroz/nrg-rs/blob/main/docs/superpowers/specs/2026-09-18-milestone-3-behaviours-and-movement-design.md)
("Movement"). nrg stage 3 adopts what this design adds.

Tiles keep deciding where the ground can be walked and remain the grid the route search runs
on. What changes is that a body is a circle at any point, a move is a straight leg between two
points, and a body reserves a circle at the end of its leg instead of a tile.

## Vocabulary

* A **body** is a character's footprint: a circle of a given radius, in tile units, centred on
  its position.
* A **leg** is one straight move at constant pace, from where a body stands to a point.
* A **reservation** is the claim a body holds on where its leg ends, or on where it stands when
  it is not moving. Other bodies may not end a leg on it.
* A **capsule** is the shape a circle sweeps when moved along a segment: every point within the
  radius of the segment.
* A **legal position** for a body is one where its circle touches no blocked tile.
* The **tile ledger** is the existing `tilemap.TileOccupancyManager`: one entity per tile.

## Constraints

These are rulings from the user, passed on by the nrg session:

1. **Additive only.** Lockstep depends on vantage and uses the tile ledger, and future
   tile-based games may too. Nothing tile-based is removed or changes behaviour. The proof is
   lockstep's test suite passing, unmodified, against the new vantage.
2. **Positions stay `float64` in tile units.** No integer millimetres.
3. **No avoidance between moving bodies.** Bodies pass through each other mid-leg. Only the leg
   end is reserved.
4. **Deterministic on one machine,** as today.
5. **Body radius is per entity.** A character's size decides how much floor it occupies.

## Summary of the additions

| Piece | Package | What it is |
|---|---|---|
| `Occupancy` | `motion` | Interface `System.Occupancy` reserves through |
| `Available`, `Claim`, `Stop` | `tilemap` | Methods making `TileOccupancyManager` an `Occupancy` |
| `CircleReservations` | `tilemap` | Per-entity circle reservation table |
| `System.CancelMove` | `motion` | Cancels an in-flight move and fixes its reservation |
| `CapsuleTouchesRectangle` | `geometry` | Exact capsule-against-box contact test |
| `WalkIsClear` | `motion` | Straight walk test over a terrain |
| `FindBodyPath` | `pathfinding` | A* without corner cutting and without reservations |
| `System.FindRoute` | `motion` | Route between two arbitrary points |
| `TightenRoute` | `motion` | Next leg end along a route |
| `MoveOptions.Duration`, `Movement.Timed` | `motion` | Legs with an explicit duration |

## Reservations

### The `Occupancy` interface

`motion.System.Occupancy` keeps its name and becomes an interface:

```go
// Occupancy records where each body stands or is headed, and refuses a move whose end another
// body holds.
type Occupancy interface {
	// Available reports whether id could claim position: nothing another entity holds is there.
	Available(id ecs.EntityId, position geometry.Vector2) bool

	// Claim reserves destination for id as it sets off from from, releasing what id held at
	// from. It reports false, and changes nothing, when another entity holds destination.
	Claim(id ecs.EntityId, from, destination geometry.Vector2) bool

	// Stop records that id halted at position before reaching destination: it drops id's claim
	// on destination and claims position.
	Stop(id ecs.EntityId, destination, position geometry.Vector2)
}
```

The interface is point-based. The tile ledger turns each point into its tile, and the circle
table uses the point with the entity's stored radius, so `MoveEntity` never needs to know a
radius.

`System` uses it this way:

* `MoveEntity` calls `Claim(id, position, destination)` once, before the at-destination check,
  and returns `MoveOutcomeDestinationOccupied` when it reports false. Today `MoveEntity` does
  four steps by hand: refuse, clear the current tile, set the current tile when already at the
  destination, set the destination tile. For the tile ledger, one `Claim` produces the same end
  state in every case, because clearing and then setting the same tile leaves it set.
* `CanReachTile` and `CanReach` ask `Available` at the tile centre and at the point
  respectively, after the terrain check.
* `CancelMove` calls `Stop`.
* The tile-only helpers (`FindTilePath`, `canEndPathOnTile`, and through them
  `FindPathBetween`, `MoveEntityTowards` and `MoveEntityTowardsArea`) route around reserved
  tiles through an unexported interface holding `GetOccupant(tilemap.TileCoord)`, which they
  type-assert `Occupancy` against. The tile ledger satisfies it, so these helpers behave exactly
  as today. The circle table does not, so under it they ignore bodies, as nrg-rs's route search
  does.

Lockstep and nrg assign `Occupancy: world.TileOccupancyManager`, a non-nil
`*tilemap.TileOccupancyManager`, which still compiles against the interface. A nil pointer
assigned to the field would now be a non-nil interface and panic on first use; no consumer
does that (checked in lockstep and nrg).

### Tile ledger methods

`*tilemap.TileOccupancyManager` gains the three methods, each turning a point into its tile with
`WorldPositionToTile`:

* `Available(id, position)`: the tile is unreserved or reserved by `id`.
* `Claim(id, from, destination)`: refuse if another entity holds the destination tile;
  otherwise clear the `from` tile, whoever holds it, and set the destination tile to `id`.
  Clearing the `from` tile unconditionally is today's `MoveEntity` behaviour, kept as it is;
  the method comment says so.
* `Stop(id, destination, position)`: clear the destination tile if `id` holds it, then set the
  position's tile to `id` if it is free or already `id`'s. This is what nrg's push does by hand
  today (`core/core_push.go`).

### Circle reservation table

`tilemap.CircleReservations` holds one circle per entity: a centre and that entity's own radius.

* `NewCircleReservations(cellSize float64)` builds an empty table over its own `SpatialGrid`.
  The grid is separate from `System.Grid`, which indexes current positions; reservations sit
  at leg ends.
* `Place(id, position, radius)` puts an entity in the table, or moves it, unconditionally. It is
  for spawning, teleports and loading saves. It panics on a negative radius.
* `Remove(id)` takes an entity out.
* `Reservation(id) (centre geometry.Vector2, radius float64, ok bool)` reads an entry back, for
  debug drawing and saves.
* `Available`, `Claim` and `Stop` satisfy `motion.Occupancy`:
  * Two circles conflict when the distance between their centres is less than the sum of their
    radii. Circles that only touch do not conflict.
  * `Available(id, position)` reports whether a circle of `id`'s radius at `position` conflicts
    with no other entity's circle.
  * `Claim(id, from, destination)` moves `id`'s circle to `destination` when it is available.
    `from` is not needed, since an entity holds exactly one circle.
  * `Stop(id, destination, position)` moves `id`'s circle to `position` unconditionally. A body
    halted mid-leg may stand on another's reservation, because bodies pass through each other;
    refusing to record where it physically is would lose it from the table.
  * All three panic when `id` was never placed, since the table cannot know its radius.
* Finding conflicts searches the grid over the square around `position` whose half-width is
  `id`'s radius plus the largest radius the table has held. That maximum only grows, which can
  widen the search a little after a large body leaves, but never misses a conflict.
* Candidates come from `SpatialGrid.GetRange`, which returns them in `EntityId` order, so
  results are deterministic.

### Cancelling a move

`System.CancelMove(id) bool` removes `id`'s `Movement` and, when `Occupancy` is set, calls
`Stop(id, destination, position)` with the cancelled destination and the current position. It
reports whether a move was in flight; with none it changes nothing. `System.Grid` needs no
update, since `Tick` keeps it at the current position.

## Straight walk test

### Capsule against a box

`geometry.CapsuleTouchesRectangle(a, b geometry.Vector2, radius float64, r geometry.Rectangle)
bool` reports whether the capsule of `radius` around segment `ab` touches the closed rectangle.
Touching counts. The test is exact: two disjoint convex shapes are closest at a vertex of one of
them, so it checks both ends against the box, the four box corners against the segment, and the
segment against the four edges, as nrg-rs's `capsule_touches_box` does. A zero-length segment is
a circle.

### Walk test over terrain

`motion.WalkIsClear(terrain pathfinding.TerrainProvider, from, to geometry.Vector2, radius
float64) bool` reports whether a body of `radius` can walk straight from `from` to `to`: whether
its capsule touches no tile that is out of bounds or not walkable. The capsule includes both end
circles, so a body in an illegal position cannot walk anywhere. `WalkIsClear(t, p, p, r)` tells
whether a body fits at `p`.

It visits only the tiles the capsule can touch, column by column. For each tile column `x` from
`floor(min(ax, bx) − radius)` to `floor(max(ax, bx) + radius)`, the capsule's points in that
column lie within `radius` of the part of the segment whose x lies in
`[x − radius, x + 1 + radius]`. So the column's candidate rows run from the lowest y of that part
of the segment minus `radius` to the highest plus `radius`. Each blocked candidate is tested
exactly with `CapsuleTouchesRectangle`. The cost grows with the leg's length, not with its
bounding box. It panics on a negative radius.

## Route search between points

### No corner cutting

`pathfinding.FindPath` lets a diagonal step pass a blocked corner when either side tile is
walkable. For a body with a radius, the segment between the two tile centres runs exactly
through that corner, so the step can never pass the walk test, and a body routed that way
stalls. `pathfinding.FindBodyPath(terrain, start, goal, maxExpansions, heuristic)` is the same
A*, allowing a diagonal only when both side tiles are walkable (nrg-rs's rule) and ignoring
reservations, since bodies do not route around each other. Both share one implementation, so
`FindPath`'s behaviour, and its fast rejections of unreachable goals, stay exactly as they are.

### `System.FindRoute`

`System.FindRoute(from, to geometry.Vector2) []geometry.Vector2` returns the points a body walks
through from `from` to `to`, in order:

1. the centre of `from`'s tile, unless `from` is exactly on it;
2. the centres of the tiles strictly between, from `FindBodyPath`;
3. the centre of `to`'s tile, unless `to` is exactly on it;
4. `to` itself.

It returns nil when no route exists: `to`'s tile is not walkable, or the search found nothing
within `MaxPathExpansions`. When `from` and `to` share a tile it returns steps 1, 3 and 4 with
the repeated centre once; when `from == to` it returns an empty slice. It uses `Terrain`,
`MaxPathExpansions` and `Heuristic`, panics when `Terrain` or `MaxPathExpansions` is not set, as
`FindTilePath` does, and records under the `"pathfinding"` phase. `FindPathBetween` does not
change.

### The route guarantee

Steps 1 and 3 are deliberate. nrg-rs's route runs straight from the last intermediate centre to
`to`. When `to` sits near a blocked corner of its tile, that last segment can fail the walk test
even though `to` is a legal position, and the body stalls one tile short forever.

With both centres included, every consecutive pair of route points passes the walk test when
the radius is under 0.5 and `from` and `to` are legal positions:

* **Centre to neighbouring centre.** A cardinal step runs half a tile from every tile that is
  not on its line. A diagonal step, with both side tiles walkable, passes the nearest other
  corner at about 0.71.
* **A point to its own tile's centre.** A blocked edge neighbour is at a distance that varies
  linearly along the segment, so its minimum is at an end, and both ends are at least the
  radius away. For a blocked corner neighbour, with the corner at the origin and the tile
  `[0,1]²`, the closest point of the segment to the corner is either an end or the foot of the
  perpendicular. A foot `f` inside the tile on a line through the centre `c` satisfies
  `f·c = |f|²`, and `f·c ≥ 0.5|f|` in that quadrant, so `|f| ≥ 0.5`.

Tests pin this, including the corner case that stalls nrg-rs.

## Route tightening

`motion.TightenRoute(terrain, from geometry.Vector2, route []geometry.Vector2, radius, maxLength
float64) (geometry.Vector2, bool)` returns where the next leg ends:

* It walks the route in order, keeping the last point `WalkIsClear` accepts from `from`.
* It stops at the first point that fails, and also right after the first point farther than
  `maxLength` from `from`. That bound keeps the cost at a handful of walk tests however long the
  route is.
* It shortens the result to `maxLength` along the straight line from `from`. Part of a clear
  capsule is clear, so the shortened leg is clear.
* It reports false when no point is clear, or when the only clear point is `from` itself.

Two differences from nrg's brief: the leg cap is built in, and the scan is bounded. A caller
wanting the uncapped point passes a large `maxLength`.

With a radius under 0.5 and a route from `FindRoute`, the route guarantee means the first point
is always clear, so tightening always makes progress.

### Large bodies

A radius of 0.5 or more (a body 1.5 m wide at nrg's scale) is allowed, but the route guarantee
no longer holds. `FindBodyPath` knows nothing of clearance and may route through a gap narrower
than the body. The walk test still never lets the body cross a blocked tile: tightening stops
short and reports false rather than returning a leg through a wall. Routing large bodies
properly needs a route search that knows how much clearance each tile has, which is left for
later.

## Legs with an explicit duration

nrg computes each leg's duration itself, from the ground under it, so a move can take a fixed
duration:

* `MoveOptions.Duration time.Duration`: when positive, the move takes exactly this long.
  `Speed` is then not needed and may be zero. The started `Movement` records
  `Speed = distance / Duration` for information, and `MoveStart.Duration` reports `Duration`.
  With `Duration` zero, `Speed` must be positive, as today.
* `Movement.Timed bool`: the move runs on the parametric path, with position a pure function of
  `Start`, `Destination` and `Elapsed / Total`, even when `Ease` is `easing.CurveLinear`.
  `MoveEntity` sets it when `Duration` is positive. Its zero value keeps today's routing, so
  every existing move, and every save decoded without it (`gob` leaves a missing field at its
  zero value), behaves as before.

The parametric path matters for nrg's determinism test, which runs a scenario once in one long
time step and once frame by frame, and compares the two. The incremental constant-speed path
sums per-tick displacements, so the two runs drift apart in the last bits of every position
mid-leg. Once a push reads a mid-leg position, the runs diverge. `Elapsed` is an integer
`time.Duration`, so it sums exactly, and the parametric position and completion tick come out
the same however time is sliced.

## Errors and panics

Following the existing `motion` convention, misconfiguration panics with context:

* a negative radius, in `WalkIsClear`, `TightenRoute` and `CircleReservations.Place`;
* an entity absent from the circle table, in its `Available`, `Claim` and `Stop`;
* neither a positive `Speed` nor a positive `Duration` in `MoveOptions`;
* `FindRoute` without `Terrain` or `MaxPathExpansions`.

Normal outcomes are not errors: an occupied destination is `MoveOutcomeDestinationOccupied`, no
route is nil, and no leg is `false`.

## Testing

* **Capsule against a box:** crossing through the middle; ending inside; running parallel to an
  edge at exactly the radius (touching) and just beyond it (clear); passing a corner just inside
  and just outside the radius; a zero-length capsule; a degenerate segment on an edge. Distances
  are chosen to be exact in binary floating point (0.25, 0.5, 0.75) so the boundary cases do not
  depend on rounding.
* **Walk test:** the corners of blocked tiles, in every diagonal direction; a body standing in a
  one-tile hole; a leg grazing out of bounds; long diagonal and axis-aligned legs checked against
  a brute-force scan of every tile in the bounding box.
* **Route search:** no step cuts a blocked corner; every route point is a tile centre except the
  last; the shared-tile, same-point and unreachable cases; `FindPath`'s existing tests unchanged.
* **Route guarantee:** a property test over random small maps, random legal `from` and `to`, and
  radii under 0.5, checking that every consecutive pair of `FindRoute` points passes the walk
  test; plus the fixed near-corner case that stalls nrg-rs.
* **Tightening:** it never returns a point whose walk fails; the cap; the lookahead bound;
  reporting false; a large body stopping short at a narrow gap.
* **Circle table:** conflicts at less than the sum of radii and none at exactly the sum, with
  unequal radii; a large body's conflict found from a small body's query; `Stop` onto another's
  circle; `Remove`; deterministic ordering.
* **Tile ledger through the interface:** `MoveEntity`'s existing tests pass unchanged; new tests
  for `Claim`, `Available` and `Stop`, including the unconditional clear of the `from` tile.
* **Cancelling:** with the tile ledger and with circles; with no move in flight.
* **Timed legs:** a leg ends exactly at its duration; one long tick and many small ticks give
  bit-identical positions mid-leg and complete on the same tick.
* **Consumers:** lockstep's and nrg's full test suites run against the new vantage through a
  temporary `replace` directive in scratch copies of those repositories. Neither repository is
  committed to.

## Documentation

* `motion/doc.go`, `tilemap/doc.go`, `pathfinding/doc.go`: the new pieces.
* `ARCHITECTURE.md`: the package map and the spatial indexing abstraction.
* `docs/performance_optimization.md`: the walk test's column scan, the circle table's
  largest-radius search margin, and `FindRoute` allocating a fresh route per call.

## Release

Tag `v0.1.29` once the checks pass, then tell the nrg session the version, the API above, and
where this design departs from its brief: tightening caps the leg itself and bounds its scan;
routes use the strict diagonal rule and include both end tiles' centres; fixed-duration legs are
parametric; the circle table stores each radius at `Place`; large bodies lose the progress
guarantee.

## Out of scope

* Avoidance between moving bodies (constraint 3).
* Integer positions for identical results across machines.
* A route search that knows each tile's clearance, for bodies of radius 0.5 or more.
* Changing the tile helpers (`FindPathBetween`, `MoveEntityTowards`, `MoveEntityTowardsArea`)
  to work in continuous space. Continuous callers compose `FindRoute`, `TightenRoute`,
  `WalkIsClear` and `MoveEntity` themselves.

## Rulings made during implementation

None yet.
