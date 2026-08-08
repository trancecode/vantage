#!/bin/bash

# Check that the graphics-free packages stay graphics-free.
#
# Ebitengine's internal/ui package aborts the process at init time when no
# display is available, so any package that links it can only be built and
# tested under a virtual display. The simulation stack (geometry, sim, tilemap,
# motion, pathfinding, util, ...) must stay linkable without one, so that games
# can run headless simulations on a server and in CI.
#
# Every package in the module is required to be graphics-free unless it is
# listed in GRAPHICS_PACKAGES below. Adding Ebitengine to a new package is
# therefore a deliberate act: it fails this check until the package is declared
# as part of the presentation stack.

set -euo pipefail

# Packages that legitimately link Ebitengine (the presentation stack and the
# demos that drive it), as import paths relative to the module root.
GRAPHICS_PACKAGES=(
    ./app
    ./asset
    ./cmd/showcasedemo
    ./render
    ./render/pixeltest
    ./scene
    ./ui
    ./visualtest/capture
)

MODULE=$(go list -m)

is_graphics_package() {
    local pkg="$1"
    local graphics
    for graphics in "${GRAPHICS_PACKAGES[@]}"; do
        if [ "$MODULE/${graphics#./}" = "$pkg" ]; then
            return 0
        fi
    done
    return 1
}

HEADLESS_PACKAGES=()
for pkg in $(go list ./...); do
    if ! is_graphics_package "$pkg"; then
        HEADLESS_PACKAGES+=("$pkg")
    fi
done

echo "Checking that ${#HEADLESS_PACKAGES[@]} packages link without Ebitengine..."

FOUND_ISSUES=0
for pkg in "${HEADLESS_PACKAGES[@]}"; do
    # -test covers the test binary too, since the acceptance is that these
    # packages can also be tested without a display. The deps go through a
    # variable rather than a pipe into grep -q: under pipefail, a grep that
    # exits on the first match can leave go list killed by SIGPIPE, and the
    # non-zero pipeline status would read as "no match".
    deps=$(go list -deps -test "$pkg")
    if grep -q 'hajimehoshi/ebiten' <<<"$deps"; then
        echo "Error: $pkg pulls Ebitengine into its dependency graph"
        echo "  Run: go list -deps -test $pkg | grep ebiten"
        FOUND_ISSUES=1
    fi
done

if [ $FOUND_ISSUES -ne 0 ]; then
    echo ""
    echo "Move the renderer-facing code to the presentation stack, or add the"
    echo "package to GRAPHICS_PACKAGES in $0 if it belongs there."
    exit 1
fi

echo "✓ No Ebitengine dependency in the graphics-free packages"

# Build and run them for real with no display and no xvfb, which is what a
# headless consumer does. A regression that only shows at link or init time
# would slip past the dependency check above.
echo "Running their tests with DISPLAY unset..."
env -u DISPLAY -u WAYLAND_DISPLAY go test "${HEADLESS_PACKAGES[@]}"

echo "✓ Graphics-free packages build and test without a display"
