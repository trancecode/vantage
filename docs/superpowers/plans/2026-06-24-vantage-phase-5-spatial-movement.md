# Vantage extraction — Phase 5: spatial and movement packages

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the three remaining generic packages — `pathfinding`, `motion`, `tilemap` — from nrg into vantage. This introduces the engine's `trancecode/ecs` dependency (used by `motion` and `tilemap`).

**Architecture:** All three are already game-agnostic in nrg and import no game package. `pathfinding` is fully self-contained (A* over a `TerrainProvider` interface, no internal or ecs imports). `motion` and `tilemap` import `geometry` (already in vantage) and `trancecode/ecs/ecs`. The extractions are mechanical: copy files, rewrite the `geometry` import path, resolve dependencies, and run the carried-over tests.

**Tech Stack:** Go 1.26.4, `github.com/trancecode/ecs/ecs`, `github.com/stretchr/testify` (test-only), the vantage `geometry` package.

## Global Constraints

* Module `github.com/trancecode/vantage`. Rewrite each moved file's `github.com/herve-quiroz/nrg/<pkg>` import to `github.com/trancecode/vantage/<pkg>`.
* Go version `1.26.4` (canonical; do not change).
* Pin `github.com/trancecode/ecs` to the exact version nrg uses — `v0.1.1-0.20260620052537-953afc80bc40` — to avoid ECS API drift. (Phase 6's MVS will reconcile if nrg later bumps it.)
* Set `GOMODCACHE=/tmp/go-mod-cache` before any Go command.
* `pathfinding` has no Ebiten dependency: test with plain `go test ./pathfinding/...`. `motion` and `tilemap` transitively import Ebiten through `geometry`→`util`, so test them under `xvfb-run -a` (their tests do not open a display, but xvfb is the safe default).
* `gofmt -l` clean on all moved files.
* Moved files change only by the import-path rewrite. No other edits.
* New module dependencies introduced this phase (document in the final summary): `github.com/trancecode/ecs` (used by `motion`/`tilemap`) and `github.com/stretchr/testify` (test-only, used by `pathfinding`/`tilemap` tests).
* Commit author: name `Claude Code`, email `herve.quiroz+claude@gmail.com`. No `Co-Authored-By`.
* Work directly on `main`; commit and push per task.
* Carry-forward: do NOT modify `util`/`geometry` (keep byte-identical to nrg).
* Source of truth for moved packages: nrg at `/home/hqz/src/nrg`.

## File structure (Phase 5)

* `pathfinding/astar.go`, `pathfinding/doc.go`, `pathfinding/astar_test.go` — copied verbatim (no internal imports to rewrite).
* `motion/motion.go`, `motion/doc.go`, `motion/motion_test.go` — copied; `geometry` import rewritten.
* `tilemap/tilemap.go`, `tilemap/tilemap_grid.go`, `tilemap/doc.go`, `tilemap/tilemap_test.go`, `tilemap/tilemap_grid_test.go` — copied; `geometry` import rewritten.

---

## Task 1: Extract `pathfinding`

`pathfinding` has no internal nrg imports and no ecs dependency, so the files move verbatim. Its test uses `testify`.

**Files:**
- Create (copied from `/home/hqz/src/nrg/pathfinding/`): `astar.go`, `doc.go`, `astar_test.go`

**Interfaces:**
- Produces: `github.com/trancecode/vantage/pathfinding` with the same exported surface as nrg's `pathfinding` (A* search and the `TerrainProvider` interface).

- [ ] **Step 1: Copy the package**

```bash
cd ~/src/vantage && mkdir -p pathfinding
cp /home/hqz/src/nrg/pathfinding/astar.go /home/hqz/src/nrg/pathfinding/doc.go /home/hqz/src/nrg/pathfinding/astar_test.go pathfinding/
```

- [ ] **Step 2: Confirm there are no nrg import paths to rewrite**

```bash
cd ~/src/vantage && grep -rn 'herve-quiroz/nrg' pathfinding/ && echo "HAS NRG IMPORTS" || echo "no nrg imports — good"
```
Expected: `no nrg imports — good`. (If any match appears, rewrite `github.com/herve-quiroz/nrg/<pkg>` → `github.com/trancecode/vantage/<pkg>` before continuing.)

- [ ] **Step 3: Resolve deps, vet, format, test**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./pathfinding/... && gofmt -l pathfinding/ && go test ./pathfinding/...
```
Expected: `go.mod`/`go.sum` gain `github.com/stretchr/testify` (and its indirect deps); vet clean; `gofmt -l` silent; tests PASS (`ok github.com/trancecode/vantage/pathfinding`).

- [ ] **Step 4: Commit**

```bash
cd ~/src/vantage
git add pathfinding/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract pathfinding package from nrg

Self-contained A* search over a TerrainProvider interface; no internal or ECS
dependency. Adds testify as a test-only dependency."
git push origin main
```

---

## Task 2: Extract `motion`

`motion` imports `geometry` (rewritten to vantage) and `trancecode/ecs/ecs`. Pin ecs to nrg's exact version.

**Files:**
- Create (copied from `/home/hqz/src/nrg/motion/`): `motion.go`, `doc.go`, `motion_test.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/geometry`, `github.com/trancecode/ecs/ecs`.
- Produces: `github.com/trancecode/vantage/motion` with the same exported surface as nrg's `motion` (`PositionComponent`, `MovingComponent`, `ProcessMovement`, etc.).

- [ ] **Step 1: Copy and rewrite the geometry import**

```bash
cd ~/src/vantage && mkdir -p motion
cp /home/hqz/src/nrg/motion/motion.go /home/hqz/src/nrg/motion/doc.go /home/hqz/src/nrg/motion/motion_test.go motion/
sed -i 's#github.com/herve-quiroz/nrg/geometry#github.com/trancecode/vantage/geometry#g' motion/*.go
grep -rn 'herve-quiroz/nrg' motion/ && echo "NRG LEFT" || echo "imports rewritten"
```
Expected: `imports rewritten`.

- [ ] **Step 2: Pin ecs to nrg's version and resolve deps**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache
go get github.com/trancecode/ecs@v0.1.1-0.20260620052537-953afc80bc40
go mod tidy
grep 'trancecode/ecs' go.mod
```
Expected: `go.mod` lists `github.com/trancecode/ecs v0.1.1-0.20260620052537-953afc80bc40`.

- [ ] **Step 3: Vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./motion/... && gofmt -l motion/ && xvfb-run -a go test ./motion/...
```
Expected: vet clean; `gofmt -l` silent; tests PASS (`ok github.com/trancecode/vantage/motion`).

- [ ] **Step 4: Commit**

```bash
cd ~/src/vantage
git add motion/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract motion package from nrg

Position/movement components and physics over the ECS. Introduces the engine's
github.com/trancecode/ecs dependency, pinned to nrg's version."
git push origin main
```

---

## Task 3: Extract `tilemap`

`tilemap` imports `geometry` (rewritten) and `trancecode/ecs/ecs` (already pinned by Task 2); its tests use `testify`.

**Files:**
- Create (copied from `/home/hqz/src/nrg/tilemap/`): `tilemap.go`, `tilemap_grid.go`, `doc.go`, `tilemap_test.go`, `tilemap_grid_test.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/geometry`, `github.com/trancecode/ecs/ecs`.
- Produces: `github.com/trancecode/vantage/tilemap` with the same exported surface as nrg's `tilemap` (`TileCoord`, coordinate conversion, `TileOccupancyManager`, `SpatialGrid`).

- [ ] **Step 1: Copy and rewrite the geometry import**

```bash
cd ~/src/vantage && mkdir -p tilemap
for f in tilemap.go tilemap_grid.go doc.go tilemap_test.go tilemap_grid_test.go; do
  cp "/home/hqz/src/nrg/tilemap/$f" tilemap/
done
sed -i 's#github.com/herve-quiroz/nrg/geometry#github.com/trancecode/vantage/geometry#g' tilemap/*.go
grep -rn 'herve-quiroz/nrg' tilemap/ && echo "NRG LEFT" || echo "imports rewritten"
```
Expected: `imports rewritten`.

- [ ] **Step 2: Resolve deps, vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./tilemap/... && gofmt -l tilemap/ && xvfb-run -a go test ./tilemap/...
```
Expected: vet clean; `gofmt -l` silent; tests PASS (`ok github.com/trancecode/vantage/tilemap`).

- [ ] **Step 3: Full-module sanity build and test**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go build ./... && xvfb-run -a go test ./...
```
Expected: every package (`app`, `asset`, `config`, `geometry`, `motion`, `pathfinding`, `render`, `scene`, `tilemap`, `ui`, `util`) builds and tests PASS.

- [ ] **Step 4: Commit**

```bash
cd ~/src/vantage
git add tilemap/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract tilemap package from nrg

Tile coordinate conversion, tile occupancy tracking, and the spatial grid over
the ECS. Completes the engine's generic package set."
git push origin main
```

---

## Self-review (Phase 5)

* **Spec coverage:** Implements the design spec's remaining package rows — `pathfinding`, `motion`, `tilemap` — and introduces the planned `trancecode/ecs` engine dependency. With this, the full generic engine package set from the spec is extracted.
* **Placeholders:** none. All three are verbatim copies with a single import-path rewrite (none for `pathfinding`).
* **Type consistency:** the only change to moved files is the `geometry` import path; their exported surfaces are unchanged, so downstream consumers (nrg in Phase 6) see the same API under the new module path.
* **Dependencies:** `ecs` is pinned to nrg's exact version; `testify` enters as a test-only dependency. No engine package imports a game package; layering stays acyclic (`motion`/`tilemap` → `geometry` → `util`; `pathfinding` standalone).
* **Deferred:** nrg migration onto all extracted packages (Phase 6).
