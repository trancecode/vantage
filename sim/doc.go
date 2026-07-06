// Package sim provides deterministic event scheduling for games built on
// vantage. It holds only scheduling and time-advancement machinery; game
// content (what an event does) stays in the consuming game.
//
// Key exports:
//   - Event: a scheduled occurrence about a single entity, ordered
//     lexicographically by Time, then Key (a client-defined discriminator),
//     then Entity, so dequeue order is a pure function of the queued set.
//   - EventQueue: a min-heap of Events with ordered read-ahead (PeekAhead),
//     in-memory snapshot and rebuild (Snapshot, Restore), and occasional
//     Reschedule/Cancel of a queued event by (entity, key).
//   - Driver: owns the game clock and advances it event by event, running
//     registered TickSystems over each interval and draining the event queue at
//     each stop through a single EventHandler, resolving same-instant cascades
//     before the clock moves.
package sim
