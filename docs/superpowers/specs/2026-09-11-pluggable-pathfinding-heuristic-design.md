# Pluggable pathfinding heuristic design

## Purpose

Let each game choose how `pathfinding.FindPath` estimates the remaining cost to
the goal, and ship a coarse cost field as one of the choices, so that terrain
faster than grass (roads) and slower than grass (forest, swamp) both shape the
estimate without the expansion blowup of dividing octile distance by the
fastest speed.

The request originates from `nrg`, whose unbounded generated world is
dominated in the Blighted Reaches by half-speed forest with impassable dead
pools, and which will stamp roads at about 2.0 into its chunks. Two
measurements frame the problem (vantage 5c863c1,
[pathfinding_performance.md](../../pathfinding_performance.md)):

* Octile distance assumes every step costs its distance. Roads break that from
  below: routes come out 10% to 73% dearer than optimal wherever a road is
  worth taking. Forest breaks it from above: the estimate falls short, so a
  1,000-tile Reaches journey expands hundreds of thousands of nodes.
* Dividing octile distance by the fastest speed restores optimal routes but
  weakens the estimate everywhere: about 0.8 x length² expansions on open
  grass, so a 100,000 budget covers about 350 tiles.

A good estimate has to know where the fast and slow ground is. The coarse cost
field learns it from the terrain itself, a cell at a time, and accepts
near-optimal rather than guaranteed-optimal routes.

## Vocabulary

* An *estimate* is the tile search's guess of the cost from a tile to the goal.
  A* orders its work by cost so far plus estimate.
* An estimate is *admissible* when it never exceeds the true remaining cost; A*
  then returns optimal routes. It is *consistent* when it drops by no more than
  a step costs across any step; A* then never finds a better route to a node it
  has already closed. vantage's A* never reopens a closed node, so an
  inconsistent estimate costs route quality rather than correctness.
* A *cell* is a square block of `CellSize` tiles on a fixed grid anchored at
  tile (0, 0). A cell's *crossing rates* are the cheapest real cost per tile of
  crossing it west to east and north to south.
* The *coarse search* is a search over cells, from the goal toward the start,
  that assigns each cell centre an estimated cost to the goal. It is resumed on
  demand whenever the tile search asks for an estimate it has not settled yet.
* The *cell budget* bounds how many cells one coarse search may settle. Past it,
  estimates fall back to octile distance.

## API

### `pathfinding.Heuristic`

```go
// Heuristic supplies the estimate FindPath orders its search by: for each
// tile, a guess of the cost of reaching the goal from it.
type Heuristic interface {
	// ForSearch returns the estimate for one search from start to goal. The
	// returned Estimate may keep per-search state and is used by one search
	// only.
	ForSearch(start, goal Coord) Estimate
}

// Estimate returns the estimated cost of reaching one search's goal from tile.
type Estimate func(tile Coord) float64
```

`FindPath` takes the heuristic as its last parameter:

```go
func FindPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) []Coord
```

A nil heuristic is octile distance, exactly as today: identical routes and
identical expansion counts. There is no exported octile heuristic, because a
second way to say "nil" is a second way to express one concept.

The heuristic only estimates. Goal rejection, occupancy, walkability, step
costs, the corner-cutting rule and the expansion budget stay inside `FindPath`,
in one place, whatever heuristic is configured. That is why there is no
whole-pathfinder interface.

`FindPath` calls `ForSearch` once per search that runs, after the up-front goal
rejections, so a rejected goal costs a heuristic nothing.

### `pathfinding.ScaledOctile`

```go
// ScaledOctile is octile distance divided by the fastest speed multiplier any
// tile reports, which never overestimates and so returns optimal routes, at
// the price of a weaker estimate everywhere.
type ScaledOctile struct {
	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. It must be positive.
	MaxSpeed float64
}
```

It replaces `MaxSpeedProvider` from 5c863c1. The fastest speed moves from a
terrain method to a value on the strategy, set where the game configures its
search. `ForSearch` panics when `MaxSpeed` is not positive. Declaring less than
the fastest tile brings back the overestimate; declaring more only costs
expansions.

It earns its place as the optimal reference the benchmarks measure against, and
as the right choice on a finite map small enough that its expansion cost fits
the budget.

### `pathfinding.CoarseCost`

The coarse cost field: see the section below for the algorithm.

```go
// CoarseCostConfig configures a CoarseCost. Every field must be set.
type CoarseCostConfig struct {
	// CellSize is the side of a cell in tiles, at least 2.
	CellSize int

	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. The coarse search divides its own focus by it, which keeps cell
	// values from missing faster ground they have not looked at yet.
	MaxSpeed float64

	// CellBudget bounds how many cells one search may settle before its
	// estimates fall back to octile distance. It is what makes the coarse
	// search return on a terrain with no edge.
	CellBudget int
}

// NewCoarseCost returns a coarse cost field over terrain. It panics when a
// config field is not set.
func NewCoarseCost(terrain TerrainProvider, config CoarseCostConfig) *CoarseCost
```

A `CoarseCost` is bound to the terrain it was built with and must be given to
`FindPath` together with that same terrain. Its cell cache is keyed per cell,
so dropping one cell later is an additive method, not a redesign.

`DefaultCoarseCellSize` and `DefaultCoarseCellBudget` are exported constants
carrying the measured defaults. The config has no zero-value defaults: a game
states its values, as it does for `motion.System.MaxPathExpansions`.

### `motion.System.Heuristic`

```go
// Heuristic estimates the remaining cost for every A* search the System
// runs. Nil is octile distance.
Heuristic pathfinding.Heuristic
```

Set where the game already builds its `System`, next to `Terrain` and
`MaxPathExpansions`.

## The coarse cost field

### Cells and crossing rates

Cell `k` covers tiles `k*CellSize` to `k*CellSize + CellSize - 1` on each axis,
and its centre sits at `k*CellSize + (CellSize-1)/2`. A cell is built the first
time anything asks for it, and never rebuilt. Building reads each of its
`CellSize²` tiles once (in bounds, walkable, speed) and runs two multi-source
Dijkstra searches restricted to the cell, with `FindPath`'s own step cost and
corner-cutting rule:

* west to east: every walkable tile of the first column starts at cost zero,
  and the search stops at the first tile of the last column it closes;
* north to south: the same from the first row to the last.

Each cost divided by `CellSize - 1` is a *crossing rate*, the cheapest cost per
tile of crossing the cell that way; a cell that cannot be crossed that way has
an infinite rate. A road running the length of a cell gives it a rate of 0.5
along the road; half-speed forest gives 2.0; a pool that blocks the cell gives
infinity. Taking the cheapest crossing rather than a median is what keeps a
3-tile road visible inside a 32-tile cell.

A cell that straddles the edge of a finite map is measured over its in-bounds
part only, since its out-of-bounds tiles are unwalkable and would otherwise
leave no tile of the last column or row to reach. West to east, the search
starts from the walkable tiles of the first column holding an in-bounds tile,
stops at the first tile of the last such column it closes, and divides by the
distance between those two columns rather than by `CellSize - 1`; north to
south likewise with rows. When only one column (or row) is in bounds, the rate
is the inverse of the highest speed among the cell's walkable tiles, and
infinite when none is walkable. A cell entirely in bounds is measured exactly
as above, so edgeless maps are unaffected.

### The coarse search

Each search runs its own coarse search over cell centres, from the goal toward
the start, and keeps it paused between requests:

* **Seeds.** The four centres surrounding the goal start at their local cost
  from the goal (see "Local cost" below).
* **Edges.** Each centre connects to its eight neighbours. An east or west edge
  costs `CellSize` times the mean of the two cells' west-east rates, north and
  south likewise with north-south rates, and a diagonal edge costs
  `√2 * CellSize` times the mean of the two cells' mean rates. An infinite edge
  is never taken. Relaxing an edge builds the neighbour cell.
* **Order.** A* toward the start cell: priority is cost from the goal plus the
  centre's octile distance to the start cell's centre divided by `MaxSpeed`.
  Dividing by the fastest speed keeps that focus from ever overestimating, so a
  settled centre's value is the cheapest coarse route to the goal, including
  one over a road the search had not reached yet.
* **Resumption.** When the tile search needs a centre's value, the coarse search
  runs until that centre is settled, the open set empties, or `CellBudget`
  centres have been settled in this search. A cell uncrossable both ways
  answers infinity without searching.

### The estimate

For a tile, the estimate takes the values of the four centres surrounding it and
blends them bilinearly by the tile's position between them. Centres that are
infinite or unsettled within the budget are left out and the remaining weights
renormalized. Within one cell of the goal's cell, the estimate is at most the
local cost straight to the goal, so the last stretch is guided by the goal
itself rather than by centres around it. When no surrounding centre has a value,
the estimate is octile distance.

Every estimate is multiplied by 1.01. On uniform ground an oblique journey has a
wide band of equally cheap routes, and an estimate that is nearly exact leaves
their priorities tied, so A* expands the band. The slight scale breaks the ties
toward the goal.

**Local cost.** The cost from a tile to a nearby point, using the rates of the
tile's own cell: `dx*rateWE + dy*rateNS - min(dx, dy) * (rateWE + rateNS) *
(1 - √2/2)`. On uniform ground it is octile distance times the rate. An
infinite rate is replaced by the other axis's rate, and by 1.0 when both are
infinite.

### Defaults

`DefaultCoarseCellSize` is 32, chosen from prototype runs over the benchmark
maps (grass, offset road, road grid with forest, and a Reaches-like map of
half-speed forest with pools 80 to 300 tiles wide), journeys of 250 to 1,000
tiles, weight `1/MaxSpeed`:

* 16-tile cells settle four times as many cells for the same ground: 3,773 on a
  1,000-tile grass journey against 985, which exhausted a 4,096-cell budget on
  the 1,000-tile Reaches journey and fell back into a 323,190-expansion flood.
* 64-tile cells gave the same route quality but built more ground: 369 chunks'
  worth on the 1,000-tile grass journey against 307, and twice the cold time.

`DefaultCoarseCellBudget` is 2,048, chosen from prototype runs of the final
formulation at 1,000 and 2,000 tiles:

* At 2,048 cells every 1,000-tile journey stays on coarse estimates (the most any
  needed was 1,808 cells, the oblique Reaches journey), fits a 100,000 budget, and
  comes out 0.0% to 0.8% above optimal. The 2,000-tile Reaches journeys run out
  of cells and fall back into octile floods of 2.0 to 2.7 million expansions.
* At 256 cells long journeys spill onto the fallback and lose what the field
  gains: the 2,000-tile offset-road route 46.7% above optimal, the oblique grid
  30.0%, and every 1,000- and 2,000-tile Reaches journey an octile flood.
* A larger budget buys longer journeys at the price of cold look-ahead. Covering
  a 2,000-tile Reaches journey takes about 5,650 settled cells, which in a lazily
  generated world means materializing about 1,640 64-tile chunks nothing else
  touched.

2,048 covers journeys well past nrg's current envelope of about 600 Reaches tiles
while capping the cold look-ahead at about 2,300 built cells. A game that needs
longer journeys raises it.

### Materialization cost of looking ahead

A cell cannot tell whether a road lies off the corridor without reading that
ground, and the focus that keeps coarse values honest (`1/MaxSpeed`) is what
decides how much ground that is: an ellipse around start and goal whose width
grows with how much slower the ground is than the fastest tile. On a world that
generates its terrain lazily, every cell read off the corridor materializes
ground the tile search never enters. In the prototype, cold 1,000-tile journeys
built cells covering 107 to 540 extra 64-tile chunks, and a 256-cell budget still
touched 82 to 138. No budget keeps a cold 2,000-tile journey to a few tens of
chunks while still improving it; the cost is paid once per region, since cells
are cached and warm journeys read no new ground.

The default stays game-agnostic: on an in-memory map a cell costs only its own
Dijkstra searches, and the materialization cost belongs to the game, which picks
its own `CellBudget` after measuring on its real terrain. The additive route to
removing that cost is a game-supplied per-cell cost source, answering a cell's
rates from knowledge the game already has (such as a world graph of roads)
without reading tiles. It is out of v0.1.21, and the field reads cells through
one internal seam that such a source would replace.

### Variants measured and rejected

Prototype figures, 32-tile cells unless stated:

* **Median or mean per cell**, as RimWorld does: not prototyped. A road is a
  small minority of a cell's tiles and vanishes from both.
* **Cheapest route through one surrounding centre** instead of blending: routes
  on the offset-road map 12.8%, 11.1% and 1.8% above optimal at 250, 500 and
  1,000 tiles, against 0.5%, 0.5% and 0.3% blended.
* **Focus weight 1.0** instead of `1/MaxSpeed`: settled grass centres before
  reaching the road they lead to, so offset-road came out 14.8% above optimal at
  1,000 tiles and the grid 14.5%.
* **No diagonal coarse edges**: oblique grid journeys 3.8%, 1.7% and 0.4% above
  optimal at 250, 500 and 1,000 tiles, against 0.4%, 0.3% and 0.2% with them.
  This answers the concern that four-neighbour travel overestimates oblique
  journeys: it does, and it costs route quality.
* **Diagonal edges costed through the cheaper side neighbour**, so an edge
  cannot cut an uncrossable corner cell: no change on the Reaches-like map
  (147,885 expansions on the oblique 1,000-tile journey either way), which is
  what showed that journey's flood came from ties, not corners.
* **An exact tile Dijkstra over the 3x3 cells around the goal**: small gains
  (0.8% to 0.4% on the 1,000-tile grid journey) for a 9-cell search on every
  call, short hops included.
* **The larger of blending and cheapest route**: fixed the oblique Reaches flood
  but moved offset-road routes to 7.7% and 5.0% above optimal at 250 and 500
  tiles.
* **No tie-breaking scale**: the oblique 1,000-tile Reaches journey expanded
  147,885 nodes. A scale of 1.001 brought it to 1,076 and 1.01 to 1,038, with
  route quality on the road maps unchanged.

## Behaviour guarantees

**Stale cells degrade estimates, never correctness.** An estimate only changes
the order in which `FindPath` expands nodes. Walkability, step costs, occupancy
and the returned path are all read from the terrain at search time, and both the
tile search and the coarse search are bounded by their budgets. A cell whose
terrain changed after it was built therefore costs route quality or extra
expansions, never a path through an unwalkable tile or a search that does not
return. The cache never invalidates today; a game with a mutation layer can
rebuild the `CoarseCost` or, once added, drop the changed cells.

**Mixed estimates cost route quality, not correctness.** When the cell budget
runs out, estimates switch from coarse values to octile distance, and the
estimate stops being consistent across that switch. Because A* never reopens a
closed node, a node closed under one regime can keep a worse route than the
other regime would have found. In the prototype, with a 1,024-cell budget, the
2,000-tile oblique grid journey came out 18.8% above optimal against 0.3% when
the budget covered it, and the 1,000-tile Reaches journey, once past the budget,
expanded 309,291 nodes against 1,001. Past the budget the search behaves like
today's octile search, route quality and floods included, which is why the
default budget is sized to cover the measured journeys.

**Deterministic.** The coarse field and search are a pure function of the
terrain, the configuration and the search's start and goal. Nothing iterates a
map in an order that affects results.

**Not safe for concurrent use.** Searches share the cell cache. This matches the
terrain a game is expected to pass: nrg's `worldgen.World` writes its chunk
cache on every query.

**Edgeless ground.** The coarse search settles at most `CellBudget` cells per
search and never scans the map. The tile search is still bounded by
`maxExpansions`.

## Consumers and migration

* vantage: `motion.System.FindTilePath` passes `System.Heuristic`; the tilemap
  test and the pathfinding tests and benchmarks pass nil or a strategy.
* nrg and lockstep reach `FindPath` only through `motion.System`, so neither
  needs an edit beyond the version bump. A game that sets nothing keeps today's
  routes.
* `MaxSpeedProvider` had no implementers in any consumer.

## Measurements

The figures in this document come from a throwaway prototype, a copy of the tile
search with the estimate swapped for a function, run once per case. They chose
the formulation and the defaults. The numbers a game should rely on come from the
committed benchmarks described under "Benchmarks", recorded in
`docs/pathfinding_performance.md` when the implementation lands.

## Rulings

1. A heuristic strategy, not a whole-pathfinder interface: occupancy, goal
   rejection and the budget stay in one place.
2. The cell cache never invalidates; the approach requires terrain that does not
   change once a cell is built. It is keyed per cell so that dropping a cell is
   additive later.
3. Not safe for concurrent use.
4. Cell size and cell budget defaults come from measurements, not from nrg's
   64-tile chunks, because vantage is game-agnostic.
5. `MaxSpeedProvider` is removed in favour of `ScaledOctile`.
6. Diagonal coarse edges, costed from the two cells' mean rates. Four-neighbour
   edges overestimate oblique travel and measurably cost route quality.
7. Estimates are scaled by 1.01 to break ties. It is a fixed constant rather
   than a config field: no measured journey needed another value, and a second
   knob would have no evidence to tune it by.
8. Past the cell budget, estimates fall back to octile distance rather than to
   the last settled values. Octile never runs away on unknown ground, and the
   budget default is sized so that measured journeys do not reach it.
9. Every strategy is benchmarked by committed benchmarks over the same map
   families and journey lengths, so a game design discussion can start from a
   number measured on demand.

## Testing

* **Default unchanged.** Every existing pathfinding test passes with a nil
  heuristic, and the existing benchmarks report the same expansion counts (257
  for the 256-tile journey, 100,000 for the exhausted budget).
* **`ScaledOctile`.** The road-detour test from 5c863c1 moves to the strategy:
  the scaled route matches Dijkstra's cost and beats octile's. `ForSearch` panics
  on a zero, negative or NaN `MaxSpeed`.
* **Crossing rates.** A uniform grass cell has rate 1.0 both ways; a road along a
  cell gives 0.5 along it; half-speed forest gives 2.0; a column of pool across
  the cell gives an infinite west-east rate and a finite north-south one.
* **Cache.** A second search over the same ground reads no tile of a cell the
  first search built, checked with a terrain that counts its queries.
* **Route quality.** On a small offset-road map the coarse route is within 2% of
  Dijkstra's cost, and on a forest map with a pool across the straight line the
  search stays a corridor: expansions within a small multiple of the path's
  length, where octile distance floods.
* **Budget.** With a cell budget of 1 a reachable goal is still found, through
  the octile fallback; a goal sealed in a pocket of an edgeless map returns no
  path under the tile budget.
* **Stale cells.** Blocking a road after its cells were built still returns a
  path whose every tile is walkable at search time.
* **Determinism.** Repeated searches over the same inputs return the same path.
* **Config.** `NewCoarseCost` panics on a cell size below 2, a non-positive or
  NaN max speed, or a non-positive cell budget.
* **`motion.System`.** `FindTilePath` hands `System.Heuristic` to the search, and
  a nil `Heuristic` returns the same paths as before.

## Benchmarks

The prototype's table is replaced by committed benchmarks, one per strategy,
over the same maps and journey lengths, so the numbers can be re-measured on
demand rather than trusted from this document:

* Maps: grass, offset road, road grid with forest (cardinal and oblique
  journeys), and the Reaches-like map (cardinal and oblique). All are edgeless
  and computed, as in `astar_roads_bench_test.go` today.
* Journey lengths: 250, 500, 1,000 and 2,000 tiles.
* Strategies: nil (octile), `ScaledOctile` at the map's fastest speed, and
  `CoarseCost` with the default config.
* Metrics per case, through `b.ReportMetric`: expansions, whether the search fits
  a 100,000 budget, route cost above optimal (optimal being `ScaledOctile` with no
  budget), and time per call under the budget. For `CoarseCost` additionally:
  cold time (a fresh field), warm time (a field that has already served the same
  journey), cells built, cells settled, and the extra 64-tile chunks the cells
  imply beyond those the tile search touches, which is what a lazily generated
  world pays to materialize.
* A separate benchmark measures the cost of building one cell and the memory a
  built cell keeps.

`docs/pathfinding_performance.md` says how to re-run each of them.

## Documentation and release

* `pathfinding/doc.go` describes the strategies and their costs.
* `docs/pathfinding_performance.md` gains the measurement table; the section
  added in 5c863c1 is rewritten around `ScaledOctile`.
* `docs/performance_optimization.md` is updated for what the coarse field leaves
  undone.
* Released as v0.1.21.
