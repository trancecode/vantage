package sim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// This file is the worked example for the savegame design
// (docs/superpowers/specs/2026-07-07-serialization-design.md): a miniature
// game saves clock + event queue + RNG + entity counter + components, restores
// into a fresh world, and both runs continue identically. The engine
// primitives round-trip through bytes; the components stay in the game's own
// representation, per the design.

// hitPoints is the test game's component: remaining health.
type hitPoints struct{ HP int }

// scar is spawned for every damage event, recording the amount. Spawning
// during handling exercises the allocation counter: after a load, the restored
// run must allocate the same ids the saved run does.
type scar struct{ Amount int }

const keyDamage = 1

// savegameGame wires a world, driver, and RNG the way a consuming game does.
type savegameGame struct {
	world  *ecs.World
	driver *Driver
	rng    *util.Rng
	hps    ecs.Accessor[hitPoints]
	scars  ecs.Accessor[scar]
}

func newSavegameGame(rng *util.Rng) *savegameGame {
	w := ecs.NewWorld()
	g := &savegameGame{
		world: w,
		rng:   rng,
		hps:   ecs.Components[hitPoints](w),
		scars: ecs.Components[scar](w),
	}
	g.driver = NewDriver(g)
	return g
}

// HandleEvent applies a random hit, records it on a freshly spawned scar
// entity, and schedules the next hit after a random delay.
func (g *savegameGame) HandleEvent(now util.Time, e Event) {
	hp, ok := g.hps.Get(e.Entity)
	if !ok {
		return
	}
	amount := g.rng.IntN(6) + 1
	hp.HP -= amount
	g.scars.Add(g.world.NewEntity(), scar{Amount: amount})
	if hp.HP > 0 {
		g.driver.Queue().Add(Event{
			Time:   now + util.Time(g.rng.IntN(4)+1),
			Entity: e.Entity,
			Key:    keyDamage,
		})
	}
}

// savegame is the game-owned save. Engine state crosses through the binary
// forms; components use the game's own representation (its choice of format).
type savegame struct {
	now     util.Time
	queue   []byte
	rng     []byte
	counter uint64
	ids     [][]byte
	hps     map[ecs.EntityId]hitPoints
	scars   map[ecs.EntityId]scar
}

func saveGame(t *testing.T, g *savegameGame) savegame {
	t.Helper()
	queue, err := g.driver.Queue().MarshalBinary()
	require.NoError(t, err)
	rng, err := g.rng.MarshalBinary()
	require.NoError(t, err)

	s := savegame{
		now:     g.driver.Now(),
		queue:   queue,
		rng:     rng,
		counter: g.world.EntityCounter(),
		hps:     collectAll(g.hps),
		scars:   collectAll(g.scars),
	}
	for id := range s.hps {
		b, err := id.MarshalBinary()
		require.NoError(t, err)
		s.ids = append(s.ids, b)
	}
	for id := range s.scars {
		b, err := id.MarshalBinary()
		require.NoError(t, err)
		s.ids = append(s.ids, b)
	}
	return s
}

func loadGame(t *testing.T, s savegame) *savegameGame {
	t.Helper()
	g := newSavegameGame(util.NewRng(0, 0))

	g.world.RestoreEntityCounter(s.counter)
	for _, b := range s.ids {
		var id ecs.EntityId
		require.NoError(t, id.UnmarshalBinary(b))
		require.NoError(t, g.world.RestoreEntity(id))
	}
	for id, hp := range s.hps {
		g.hps.Add(id, hp)
	}
	for id, sc := range s.scars {
		g.scars.Add(id, sc)
	}

	require.NoError(t, g.rng.UnmarshalBinary(s.rng))
	g.driver.RestoreNow(s.now)
	queue := NewEventQueue()
	require.NoError(t, queue.UnmarshalBinary(s.queue))
	g.driver.RestoreQueue(queue)
	return g
}

// collectAll copies an accessor's contents into a map for saving or comparing.
func collectAll[C any](a ecs.Accessor[C]) map[ecs.EntityId]C {
	out := map[ecs.EntityId]C{}
	for id, c := range a.All() {
		out[id] = *c
	}
	return out
}

func TestSavegameRoundTripResumesIdenticalRun(t *testing.T) {
	original := newSavegameGame(util.NewRng(11, 47))
	for i := range 3 {
		id := original.world.NewEntity()
		original.hps.Add(id, hitPoints{HP: 100})
		original.driver.Queue().Add(Event{Time: util.Time(i + 1), Entity: id, Key: keyDamage})
	}
	original.driver.RunUntil(util.Time(10))

	loaded := loadGame(t, saveGame(t, original))

	original.driver.RunUntil(util.Time(25))
	loaded.driver.RunUntil(util.Time(25))

	assert.Equal(t, collectAll(original.hps), collectAll(loaded.hps),
		"hit points must match after resuming from a save")
	assert.Equal(t, collectAll(original.scars), collectAll(loaded.scars),
		"scar entities must match, ids included: allocation resumes identically")
	assert.Equal(t, original.world.EntityCounter(), loaded.world.EntityCounter())
	assert.Equal(t,
		original.driver.Queue().PeekAhead(original.driver.Queue().Len()),
		loaded.driver.Queue().PeekAhead(loaded.driver.Queue().Len()),
		"pending events must match in dequeue order")
	assert.Equal(t, original.rng.Uint64(), loaded.rng.Uint64(),
		"the random sequence must continue identically")
}

// TestSavegameRoundTripMidRunHasWorkLeft pins the scenario shape: the save
// happens mid-run (events still pending, fighters still alive), so the
// round-trip above proves something.
func TestSavegameRoundTripMidRunHasWorkLeft(t *testing.T) {
	g := newSavegameGame(util.NewRng(11, 47))
	for i := range 3 {
		id := g.world.NewEntity()
		g.hps.Add(id, hitPoints{HP: 100})
		g.driver.Queue().Add(Event{Time: util.Time(i + 1), Entity: id, Key: keyDamage})
	}
	g.driver.RunUntil(util.Time(10))
	require.NotZero(t, g.driver.Queue().Len(), "events must still be pending at the save point")

	g.driver.RunUntil(util.Time(25))
	require.NotZero(t, g.driver.Queue().Len(), "events must still be pending at the comparison point")
	for _, hp := range collectAll(g.hps) {
		require.Positive(t, hp.HP, "fighters must still be alive at the comparison point")
	}
}
