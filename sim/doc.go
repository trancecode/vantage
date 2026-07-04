// Package sim provides deterministic event scheduling for games built on
// vantage. It holds only scheduling and time-advancement machinery; game
// content (what an event does) stays in the consuming game.
//
// Key exports:
//   - EventQueue: a generic min-heap whose dequeue order is a pure function of
//     the queued set, ordered lexicographically by event time then a
//     caller-supplied tie-break, so insertion order cannot change outcomes.
//   - Driver: owns the game clock and advances it event by event, running
//     registered TickSystems over each elapsed interval and draining registered
//     EventSources at each stop until every source is quiet at that instant.
package sim
