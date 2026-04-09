# spriteFlood

spriteFlood is a sprite stress-test game written in Go.
The player uses the gamepad left analog stick to pull falling sprites to the target side of the screen.

## What It Is

- High-volume sprite simulation and rendering benchmark
- Shared core game logic in `internal/core`
- Multiple rendering backends:
  - Ebiten
  - SDL2
  - Raylib

## Run

Ebiten:

go run ./cmd/ebiten

## Controls

- Left analog stick: pull sprites left/right
- Up Arrow: add 10k sprites (up to 100k)
- Down Arrow: remove 10k sprites (down to 10k)
- G key: toggle pixel/gopher sprite
