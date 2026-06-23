# Vantage extraction — Phase 1: foundation and tooling

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the `github.com/trancecode/vantage` Go module with build/test tooling, continuous integration, and the two dependency-free foundation packages (`util`, `geometry`) extracted from nrg, all green.

**Architecture:** Vantage is a reusable 2D game engine extracted from nrg. This first phase creates the module shell and moves the foundation layer, which has no dependency on any other engine package. `util` depends only on Ebitengine and zerolog; `geometry` depends only on `util`. Later phases build the graphics core, scene/app layer, config service, spatial/movement packages, then repoint nrg.

**Tech Stack:** Go 1.26.4, Ebitengine v2 (`github.com/hajimehoshi/ebiten/v2` v2.9.9), zerolog, Task (taskfile.dev), golangci-lint/staticcheck, xvfb for headless tests, GitHub Actions.

## Global Constraints

* Module path: `github.com/trancecode/vantage`.
* Go version: `1.26.4`. `go.mod` `go` directive is canonical; every other Go-version reference must equal it.
* License: MIT for code (`LICENSE` at repo root). Bundled fonts (later phases) are OFL with `OFL.txt` beside each font.
* Set `GOMODCACHE=/tmp/go-mod-cache` before any Go command (the default cache may be read-only).
* Ebiten-dependent packages (`util` and everything downstream) must be tested under a virtual display: use `task test:headless` (xvfb), never bare `go test`.
* Commit author: name `Claude Code`, email `herve.quiroz+claude@gmail.com`. No `Co-Authored-By` line.
* Work directly on `main` (interactive session); commit and push per task.
* Source of truth for the packages being moved is the nrg working copy at `/home/hqz/src/nrg`.

---

## Phase roadmap (context only — implement Phase 1 here)

This plan implements Phase 1. Subsequent phases each get their own complete plan, written just before execution because each depends on the previous phase's outcomes and on decisions the spec defers to implementation.

* **Phase 1 (this plan): Foundation and tooling.** Module skeleton, Taskfile, CI, `util`, `geometry`.
* **Phase 2: Graphics core.** `render` (camera transform, sprite, animation, text mechanics) with the sprite catalog removed and fonts injected; embed the two OFL fonts (Google Sans Code from `googlefonts/googlesans-code` `fonts/variable/`; Google Sans Flex from Google Fonts / Fontsource, which has no dedicated googlefonts repo); split `Camera` (transform) from `CameraController` (input); `ui` with injected fonts.
* **Phase 3: Scene and app.** `scene` package with `type SceneName string`, the `Manager`, and the `App` (implements `ebiten.Game`, owns window setup, `RunGame`, screenshot capture, watchdog, `exitAfter`).
* **Phase 4: Config service.** Layered loader (embedded engine `settings.toml` → game-registered defaults → local `settings.toml` → `--config_override`), reflection-based `section.key` routing, engine settings struct, engine flags; migrate the package-level flags currently in `util` (`DebugMode`) and `render` (`use_placeholder_sprite_images`) into config.
* **Phase 5: Spatial and movement.** `pathfinding`, `motion`, `tilemap` (these add the `trancecode/ecs` dependency).
* **Phase 6: nrg migration.** Repoint nrg to import vantage, move nrg's sprite catalog into nrg `data`, reduce nrg `game` to a thin `App` constructor, delete nrg's local copies of moved packages, verify nrg builds, headless tests pass, and a visual check matches.

---

## File structure (Phase 1)

* `go.mod`, `go.sum` — module definition and lockfile.
* `LICENSE` — MIT.
* `.gitignore` — Go build artifacts.
* `README.md` — one-paragraph repo description.
* `Taskfile.yml` — build/test/lint/setup targets.
* `scripts/check-multiple-blank-lines.sh` — lint helper (copied from nrg).
* `.github/workflows/go.yml` — vet, build, headless test on push/PR.
* `util/*.go` — logging, `Time`, `PriorityQueue`, debug HTTP server, `Watchdog`, number helpers (copied from nrg, no import changes — `util` has no internal deps).
* `geometry/*.go` — `Vector2`, `Rectangle` (copied from nrg, internal import `nrg/util` rewritten to `vantage/util`).
* `geometry/geometry_vector_test.go` — new smoke test (nrg's `geometry` ships no tests).

---

## Task 1: Module skeleton and tooling

**Files:**
- Create: `~/src/vantage/go.mod`
- Create: `~/src/vantage/LICENSE`
- Create: `~/src/vantage/.gitignore`
- Create: `~/src/vantage/README.md`
- Create: `~/src/vantage/Taskfile.yml`
- Create: `~/src/vantage/scripts/check-multiple-blank-lines.sh`

**Interfaces:**
- Produces: a buildable empty module; the `task` targets `deps`, `build`, `test`, `test:headless`, `vet`, `lint`, `setup` used by every later task.

- [ ] **Step 1: Create `go.mod`**

```
module github.com/trancecode/vantage

go 1.26.4
```

- [ ] **Step 2: Create `LICENSE` (MIT)**

```
MIT License

Copyright (c) 2026 trancecode

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: Create `.gitignore`**

```
# Binaries and build output
*.exe
*.test
*.out
/vantage

# Go workspace / cache
go.work
go.work.sum

# Editor
.vscode/
.idea/
```

- [ ] **Step 4: Create `README.md`**

```markdown
# Vantage

A reusable 2D game engine for Go, built on [Ebitengine](https://ebitengine.org/)
and the [`trancecode/ecs`](https://github.com/trancecode/ecs) entity-component-system
module. Extracted from the NRG game.

* Engine code: MIT (see `LICENSE`).
* Bundled fonts: SIL Open Font License (see each font's `OFL.txt`).
```

- [ ] **Step 5: Create `Taskfile.yml`**

```yaml
# https://taskfile.dev
version: '3'

vars:
  GOFLAGS: -buildvcs=false

tasks:
  default:
    desc: Show available tasks
    cmds:
      - task --list

  deps:
    desc: Download and verify dependencies
    cmds:
      - go mod download
      - go mod verify

  build:
    desc: Build all packages
    cmds:
      - GOFLAGS="{{.GOFLAGS}}" go build -v ./...

  vet:
    desc: Run go vet for static analysis
    cmds:
      - go vet ./...

  test:
    desc: Run all tests
    cmds:
      - go test -v ./...
    env:
      CGO_ENABLED: 1

  test:headless:
    desc: Run tests with a virtual display (for Ebiten/CI)
    cmds:
      - xvfb-run -a go test -v ./...

  lint:
    desc: Run linters and helper checks
    cmds:
      - task: vet
      - |
        if command -v staticcheck >/dev/null 2>&1; then
          staticcheck ./...
        else
          echo "staticcheck not installed, skipping..."
        fi
      - |
        if command -v golangci-lint >/dev/null 2>&1; then
          golangci-lint run --timeout=5m
        else
          echo "golangci-lint not installed, skipping..."
        fi
      - xvfb-run -a go test -race -short ./...
      - ./scripts/check-multiple-blank-lines.sh

  setup:
    desc: Download dependencies for a fresh checkout
    cmds:
      - task: deps

  install:tools:
    desc: Install development tools
    cmds:
      - go install honnef.co/go/tools/cmd/staticcheck@latest
      - go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

- [ ] **Step 6: Copy the blank-line check script from nrg and make it executable**

Run:
```bash
mkdir -p ~/src/vantage/scripts
cp /home/hqz/src/nrg/scripts/check-multiple-blank-lines.sh ~/src/vantage/scripts/
chmod +x ~/src/vantage/scripts/check-multiple-blank-lines.sh
```

- [ ] **Step 7: Verify the module builds (no packages yet is fine)**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go build ./... && echo OK
```
Expected: prints `OK` (no packages to build yet, exit 0).

- [ ] **Step 8: Commit**

```bash
cd ~/src/vantage
git add go.mod LICENSE .gitignore README.md Taskfile.yml scripts/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add module skeleton, license, and build tooling"
git push origin main
```

---

## Task 2: Extract the `util` package

`util` has no internal nrg dependencies (it imports only Ebitengine, ebitenutil, and zerolog), so the files move verbatim with no import rewriting. It carries its existing tests.

**Files:**
- Create: `~/src/vantage/util/*.go` (copied from `/home/hqz/src/nrg/util/`)
- Test: `~/src/vantage/util/util_http_test.go`, `util_log_test.go`, `util_priorityqueue_test.go`, `util_time_test.go`, `util_watchdog_test.go` (copied)

**Interfaces:**
- Produces: `github.com/trancecode/vantage/util` with the same exported surface as nrg's `util` (logging helpers, `Time`, `PriorityQueue`, debug HTTP server, `Watchdog`, number helpers, and the package-level `DebugMode` flag var — left as-is in this phase; it migrates to config in Phase 4).

- [ ] **Step 1: Copy the package**

Run:
```bash
mkdir -p ~/src/vantage/util
cp /home/hqz/src/nrg/util/*.go ~/src/vantage/util/
```

- [ ] **Step 2: Confirm no internal import rewrite is needed**

Run:
```bash
grep -l 'herve-quiroz/nrg' ~/src/vantage/util/*.go || echo "no nrg imports — good"
```
Expected: prints `no nrg imports — good`. If any file matches, rewrite `github.com/herve-quiroz/nrg/<pkg>` to `github.com/trancecode/vantage/<pkg>` before continuing.

- [ ] **Step 3: Resolve dependencies**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy
```
Expected: `go.mod`/`go.sum` gain `github.com/hajimehoshi/ebiten/v2` and `github.com/rs/zerolog` (plus their indirect deps). Confirm the `go` directive in `go.mod` is still `1.26.4`.

- [ ] **Step 4: Run the package tests under a virtual display**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./util/...
```
Expected: PASS (`ok  github.com/trancecode/vantage/util`).

- [ ] **Step 5: Vet**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./util/...
```
Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
cd ~/src/vantage
git add util/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract util package from nrg"
git push origin main
```

---

## Task 3: Extract the `geometry` package

`geometry` imports `nrg/util`, which becomes `vantage/util`. nrg ships no tests for `geometry`, so this task adds a small smoke test to confirm the package works under the new module path.

**Files:**
- Create: `~/src/vantage/geometry/*.go` (copied from `/home/hqz/src/nrg/geometry/`, import rewritten)
- Test: `~/src/vantage/geometry/geometry_vector_test.go` (new)

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/util`.
- Produces: `github.com/trancecode/vantage/geometry` with `Vector2` and `Rectangle` (same exported surface as nrg's `geometry`).

- [ ] **Step 1: Copy the package**

Run:
```bash
mkdir -p ~/src/vantage/geometry
cp /home/hqz/src/nrg/geometry/*.go ~/src/vantage/geometry/
```

- [ ] **Step 2: Rewrite the internal import path**

Run:
```bash
sed -i 's#github.com/herve-quiroz/nrg/#github.com/trancecode/vantage/#g' ~/src/vantage/geometry/*.go
grep -rn 'herve-quiroz/nrg' ~/src/vantage/geometry/ && echo "STILL HAS NRG IMPORTS" || echo "imports rewritten"
```
Expected: prints `imports rewritten`.

- [ ] **Step 3: Inspect the exported API the smoke test will use**

Run:
```bash
grep -nE '^func (New)?Vector2|^func .*Vector2.*\) (Add|Sub|Length|Scale)' ~/src/vantage/geometry/geometry_vector.go | head
```
Use the real constructor and method names from the output in the next step. The test below assumes `NewVector2(x, y float64) Vector2` and an `Add` method returning a `Vector2`; adjust names to match the grep output if they differ.

- [ ] **Step 4: Write the smoke test**

Create `~/src/vantage/geometry/geometry_vector_test.go`:
```go
package geometry

import "testing"

func TestVector2Add(t *testing.T) {
	a := NewVector2(1, 2)
	b := NewVector2(3, 4)
	got := a.Add(b)
	want := NewVector2(4, 6)
	if got != want {
		t.Fatalf("Add() = %v, want %v", got, want)
	}
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go test ./geometry/...
```
Expected: PASS (`ok  github.com/trancecode/vantage/geometry`). If it fails to compile because the constructor or method names differ, fix the test to match the names found in Step 3, then re-run.

- [ ] **Step 6: Vet**

Run:
```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./geometry/...
```
Expected: no output, exit 0.

- [ ] **Step 7: Commit**

```bash
cd ~/src/vantage
git add geometry/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract geometry package from nrg"
git push origin main
```

---

## Task 4: Continuous integration workflow

Now that the module has buildable packages, add a GitHub Actions workflow mirroring nrg's: install system packages for Ebiten/CGO, set up Go 1.26.4, vet, build, and run tests under xvfb.

**Files:**
- Create: `~/src/vantage/.github/workflows/go.yml`

**Interfaces:**
- Produces: CI that runs on push/PR to `main` and gates the repo green.

- [ ] **Step 1: Create the workflow**

Create `~/src/vantage/.github/workflows/go.yml`:
```yaml
name: Go

on:
  push:
    branches: [ "main" ]
    paths-ignore:
      - '**.md'
      - 'LICENSE'
      - 'docs/**'
  pull_request:
    branches: [ "main" ]
    paths-ignore:
      - '**.md'
      - 'LICENSE'
      - 'docs/**'

permissions:
  contents: read

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install system packages
        run: |
          sudo apt-get update
          sudo apt-get install -y \
            libc6-dev libgl1-mesa-dev libxcursor-dev libxi-dev \
            libxinerama-dev libxrandr-dev libxxf86vm-dev \
            libasound2-dev pkg-config xorg-dev xvfb

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26.4'
          cache: true

      - name: Run go vet
        run: go vet ./...

      - name: Build
        run: go build -v ./...
        env:
          GOFLAGS: "-buildvcs=false"

      - name: Test
        env:
          CI: true
        run: xvfb-run -a go test -v ./...
```

- [ ] **Step 2: Verify the workflow is valid YAML and references the canonical Go version**

Run:
```bash
grep "go-version: '1.26.4'" ~/src/vantage/.github/workflows/go.yml && echo "version matches go.mod"
```
Expected: prints `version matches go.mod`.

- [ ] **Step 3: Commit and confirm CI passes**

```bash
cd ~/src/vantage
git add .github/workflows/go.yml
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add Go CI workflow"
git push origin main
```
Then verify the run:
```bash
gh run watch --repo trancecode/vantage $(gh run list --repo trancecode/vantage --workflow go.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```
Expected: the `Go` workflow concludes `success`. If it fails, read the logs with `gh run view --log-failed` and fix before proceeding.

---

## Self-review (Phase 1)

* **Spec coverage:** This phase covers the spec's "module and repository," "licensing" (MIT `LICENSE`; OFL handled with fonts in Phase 2), the `util` and `geometry` rows of the package table, and the Go-tooling conventions imported into `CLAUDE.md`. Fonts, asset injection, scene/config/camera, spatial packages, and nrg migration are explicitly later phases.
* **Placeholders:** none. The one variable point (exact `Vector2` method names) is handled by Step 3 of Task 3 inspecting the real API before the test is written.
* **Type consistency:** `util` moves verbatim (no signature changes); `geometry`'s only change is the import path. The smoke test depends on names verified at implementation time.
* **Version consistency:** `go.mod` `1.26.4` and the workflow `go-version` are the only two Go-version surfaces and both are pinned to `1.26.4`.
