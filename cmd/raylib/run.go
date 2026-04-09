//go:build raylib

package main

import (
	"fmt"
	"math"
	"os"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"spriteflood/internal/core"
)

const (
	spriteStep = 10000
	minSprites = 10000
	maxSprites = 100000
)

func run(cfg core.Config) error {
	w := int32(cfg.ScreenWidth)
	h := int32(cfg.ScreenHeight)

	rl.InitWindow(w, h, "spriteFlood - Raylib")
	if !rl.IsWindowReady() {
		return fmt.Errorf("failed to initialize raylib window")
	}
	defer rl.CloseWindow()

	// Target 60 FPS for consistent performance benchmark
	//rl.SetTargetFPS(60)

	game := core.NewGame(cfg)
	spriteViews := make([]core.SpriteView, 0, cfg.SpriteCount)

	gopher, hasGopher := loadGopherTexture("gopher.png")
	if hasGopher {
		defer rl.UnloadTexture(gopher)
	}
	useGopher := hasGopher

	var updateMSAvg float64
	var drawMSAvg float64
	var lastAxisX float64
	var hasGamepad bool
	activePadID := -1
	activeAxisID := int32(rl.GamepadAxisLeftX)
	var upsApprox float64
	var drawsApprox float64
	perfWindowStart := time.Now()
	updatesInWindow := 0
	drawsInWindow := 0

	// Pre-compute bar dimensions
	halfW := w / 2
	barW := int32(float64(w) * 0.8)
	barX := (w - barW) / 2
	barY := int32(16)
	barH := int32(10)

	for !rl.WindowShouldClose() {
		updateStart := time.Now()

		if rl.IsKeyPressed(rl.KeyG) {
			if hasGopher {
				useGopher = !useGopher
			}
		}
		if rl.IsKeyPressed(rl.KeyUp) {
			next := game.Stats().SpriteCount + spriteStep
			if next > maxSprites {
				next = maxSprites
			}
			game.SetSpriteCount(next)
		}
		if rl.IsKeyPressed(rl.KeyDown) {
			next := game.Stats().SpriteCount - spriteStep
			if next < minSprites {
				next = minSprites
			}
			game.SetSpriteCount(next)
		}

		axisX, padConnected, nextPadID, nextAxisID := readPrimaryGamepadAxisX(activePadID, activeAxisID)
		hasGamepad = padConnected
		lastAxisX = axisX
		activePadID = nextPadID
		activeAxisID = nextAxisID
		game.Update(1.0/60.0, axisX)
		updatesInWindow++
		updateMSAvg = ema(updateMSAvg, float64(time.Since(updateStart).Microseconds())/1000.0, 0.12)

		drawStart := time.Now()
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(12, 14, 24, 255))

		// Visual split indicator (render once with pre-computed halfW)
		rl.DrawRectangle(0, 0, halfW, h, rl.NewColor(20, 70, 80, 24))
		rl.DrawRectangle(halfW, 0, halfW, h, rl.NewColor(80, 30, 30, 24))

		// Render sprites (culling happens inside draw functions)
		spriteViews = game.SnapshotInto(spriteViews)
		if useGopher && hasGopher {
			drawSpritesTexture(spriteViews, gopher)
		} else {
			drawSpritesRect(spriteViews)
		}

		stats := game.Stats()
		directionProgress := game.DirectionProgress()
		drawProgressBar(barW, barX, barY, barH, directionProgress)
		drawHUD(stats, useGopher, hasGopher, hasGamepad, lastAxisX, activePadID, activeAxisID, updateMSAvg, drawMSAvg, upsApprox, drawsApprox)

		rl.EndDrawing()
		drawsInWindow++
		drawMSAvg = ema(drawMSAvg, float64(time.Since(drawStart).Microseconds())/1000.0, 0.12)

		elapsed := time.Since(perfWindowStart)
		if elapsed >= time.Second {
			seconds := elapsed.Seconds()
			upsApprox = float64(updatesInWindow) / seconds
			drawsApprox = float64(drawsInWindow) / seconds
			perfWindowStart = time.Now()
			updatesInWindow = 0
			drawsInWindow = 0
		}
	}

	return nil
}

func drawSpritesRect(sprites []core.SpriteView) {
	for _, s := range sprites {
		// Screen culling: skip sprites completely out of view
		size := float32(s.Size * s.Scale)
		x := float32(s.X)
		y := float32(s.Y)
		halfSize := size * 0.5

		// Simple AABB culling (check if sprite bounds are within screen)
		screenW := float32(1280)
		screenH := float32(720)
		if x+halfSize < 0 || x-halfSize > screenW || y+halfSize < 0 || y-halfSize > screenH {
			continue
		}

		rect := rl.NewRectangle(x, y, size, size)
		origin := rl.NewVector2(halfSize, halfSize)
		tint := rl.NewColor(s.Tint.R, s.Tint.G, s.Tint.B, s.Tint.A)
		rotationDeg := float32(s.Angle * 180.0 / math.Pi)
		rl.DrawRectanglePro(rect, origin, rotationDeg, tint)
	}
}

func drawSpritesTexture(sprites []core.SpriteView, texture rl.Texture2D) {
	baseW := float32(texture.Width)
	baseH := float32(texture.Height)
	maxDim := baseW
	if baseH > maxDim {
		maxDim = baseH
	}
	if maxDim <= 0 {
		maxDim = 1
	}

	src := rl.NewRectangle(0, 0, baseW, baseH)
	screenW := float32(1280)
	screenH := float32(720)

	for _, s := range sprites {
		target := float32(s.Size * s.Scale)
		scale := target / maxDim
		dstW := baseW * scale
		dstH := baseH * scale
		x := float32(s.X)
		y := float32(s.Y)
		halfW := dstW * 0.5
		halfH := dstH * 0.5

		// Screen culling: skip sprites completely out of view
		if x+halfW < 0 || x-halfW > screenW || y+halfH < 0 || y-halfH > screenH {
			continue
		}

		dst := rl.NewRectangle(x, y, dstW, dstH)
		origin := rl.NewVector2(halfW, halfH)
		tint := rl.NewColor(s.Tint.R, s.Tint.G, s.Tint.B, s.Tint.A)
		rotationDeg := float32(s.Angle * 180.0 / math.Pi)
		rl.DrawTexturePro(texture, src, dst, origin, rotationDeg, tint)
	}
}

func drawProgressBar(barW int32, barX int32, barY int32, barH int32, progress float64) {
	rl.DrawRectangle(barX, barY, barW, barH, rl.NewColor(44, 44, 60, 255))
	fillW := int32(float64(barW) * progress)
	if fillW > 0 {
		rl.DrawRectangle(barX, barY, fillW, barH, rl.NewColor(250, 200, 90, 255))
	}
}

func drawHUD(stats core.Stats, useGopher bool, hasGopher bool, hasGamepad bool, axisX float64, padID int, axisID int32, updateMS float64, drawMS float64, ups float64, draws float64) {
	spriteMode := "PIXEL"
	if useGopher && hasGopher {
		spriteMode = "GOPHER.PNG"
	}
	if !hasGopher {
		spriteMode = "PIXEL (gopher.png not found)"
	}
	padInfo := "none"
	if hasGamepad {
		padInfo = fmt.Sprintf("id=%d axis=%d", padID, axisID)
	}

	lines := []string{
		"spriteFlood (Raylib)",
		fmt.Sprintf("Current target: %s", stats.TargetDirection),
		fmt.Sprintf("Score: %d | Misses: %d", stats.Score, stats.Missed),
		fmt.Sprintf("Sprites: %d", stats.SpriteCount),
		fmt.Sprintf("Sprite mode: %s", spriteMode),
		fmt.Sprintf("Analog X: %.2f", axisX),
		fmt.Sprintf("Controller connected: %v", hasGamepad),
		fmt.Sprintf("Controller readout: %s", padInfo),
		fmt.Sprintf("FPS: %d | UPS aprox: %.1f | Draw/s aprox: %.1f", rl.GetFPS(), ups, draws),
		fmt.Sprintf("Update(ms): %.3f | Draw(ms): %.3f", updateMS, drawMS),
		"",
		"Controls:",
		"- Left analog: pull sprites",
		"- Up Arrow: +10k sprites (max 100k)",
		"- Down Arrow: -10k sprites (min 10k)",
		"- G key: toggle Pixel/Gopher",
	}

	x := int32(20)
	y := int32(36)
	fontSize := int32(20)
	lineHeight := int32(22)
	for i, line := range lines {
		rl.DrawText(line, x, y+int32(i)*lineHeight, fontSize, rl.RayWhite)
	}
}

func readPrimaryGamepadAxisX(activePadID int, activeAxisID int32) (float64, bool, int, int32) {
	const deadZone = 0.18
	const maxPads = 8

	// Fast path: check if active pad is still valid and has strong input
	if activePadID >= 0 && activePadID < maxPads && rl.IsGamepadAvailable(int32(activePadID)) {
		v := rl.GetGamepadAxisMovement(int32(activePadID), activeAxisID)
		if math.Abs(float64(v)) >= deadZone {
			return float64(v), true, activePadID, activeAxisID
		}

		// Try other axes on same pad before searching other pads
		preferred := []int32{int32(rl.GamepadAxisLeftX), int32(rl.GamepadAxisRightX), 0, 2}
		for _, axis := range preferred {
			if axis == activeAxisID {
				continue
			}
			v = rl.GetGamepadAxisMovement(int32(activePadID), axis)
			if math.Abs(float64(v)) >= deadZone {
				return float64(v), true, activePadID, axis
			}
		}
	}

	// Slow path: search all pads (only when active pad is lost or invalid)
	bestValue := float32(0)
	bestPad := -1
	bestAxis := int32(rl.GamepadAxisLeftX)
	bestStrength := 0.0

	preferred := []int32{int32(rl.GamepadAxisLeftX), int32(rl.GamepadAxisRightX), 0, 2}

	for pad := int32(0); pad < maxPads; pad++ {
		if !rl.IsGamepadAvailable(pad) {
			continue
		}

		for _, axis := range preferred {
			v := rl.GetGamepadAxisMovement(pad, axis)
			strength := math.Abs(float64(v))
			if strength > bestStrength {
				bestStrength = strength
				bestValue = v
				bestPad = int(pad)
				bestAxis = axis
			}
		}
	}

	if bestPad >= 0 {
		if bestStrength < deadZone {
			return 0, true, bestPad, bestAxis
		}
		return float64(bestValue), true, bestPad, bestAxis
	}

	return 0, false, -1, int32(rl.GamepadAxisLeftX)
}

func loadGopherTexture(path string) (rl.Texture2D, bool) {
	if _, err := os.Stat(path); err != nil {
		return rl.Texture2D{}, false
	}
	texture := rl.LoadTexture(path)
	if texture.ID == 0 {
		return rl.Texture2D{}, false
	}
	return texture, true
}

func ema(prev, sample, alpha float64) float64 {
	if prev == 0 {
		return sample
	}
	return prev*(1.0-alpha) + sample*alpha
}
