//go:build ebiten

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"spriteflood/internal/core"
)

const (
	spriteStep                 = 10000
	minSprites                 = 10000
	maxSprites                 = 100000
	maxSpritesPerTriangleBatch = 16383
)

type app struct {
	logic           *core.Game
	spritePixel     *ebiten.Image
	spriteGopher    *ebiten.Image
	useGopher       bool
	leftOverlay     *ebiten.Image
	rightOverlay    *ebiten.Image
	progressBg      *ebiten.Image
	progressFillMax *ebiten.Image
	spriteViews     []core.SpriteView
	progressBarW    int
	progressBarX    int
	progressBarY    int
	updateMSAvg     float64
	drawMSAvg       float64
	lastAxisX       float64
	hasGamepad      bool
	triVertices     []ebiten.Vertex
	triIndices      []uint16
}

func run(cfg core.Config) error {
	ebiten.SetWindowTitle("spriteFlood - Ebiten")
	ebiten.SetWindowSize(int(cfg.ScreenWidth), int(cfg.ScreenHeight))
	ebiten.SetTPS(60)
	return ebiten.RunGame(newApp(cfg))
}

func newApp(cfg core.Config) *app {
	pixel := ebiten.NewImage(1, 1)
	pixel.Fill(color.White)

	half := int(cfg.ScreenWidth * 0.5)
	leftOverlay := ebiten.NewImage(half, int(cfg.ScreenHeight))
	rightOverlay := ebiten.NewImage(half, int(cfg.ScreenHeight))
	leftOverlay.Fill(color.RGBA{R: 20, G: 70, B: 80, A: 24})
	rightOverlay.Fill(color.RGBA{R: 80, G: 30, B: 30, A: 24})

	barWidth := int(cfg.ScreenWidth * 0.8)
	progressBg := ebiten.NewImage(barWidth, 10)
	progressBg.Fill(color.RGBA{R: 44, G: 44, B: 60, A: 255})
	progressFill := ebiten.NewImage(barWidth, 10)
	progressFill.Fill(color.RGBA{R: 250, G: 200, B: 90, A: 255})

	gopherImage := loadGopherImage("gopher.png")

	return &app{
		logic:           core.NewGame(cfg),
		spritePixel:     pixel,
		spriteGopher:    gopherImage,
		leftOverlay:     leftOverlay,
		rightOverlay:    rightOverlay,
		progressBg:      progressBg,
		progressFillMax: progressFill,
		progressBarW:    barWidth,
		progressBarX:    int((cfg.ScreenWidth - float64(barWidth)) * 0.5),
		progressBarY:    16,
		triVertices:     make([]ebiten.Vertex, maxSpritesPerTriangleBatch*4),
		triIndices:      makeTriangleIndices(maxSpritesPerTriangleBatch),
	}
}

func (a *app) Update() error {
	start := time.Now()

	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		a.useGopher = !a.useGopher
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		stats := a.logic.Stats()
		next := stats.SpriteCount + spriteStep
		if next > maxSprites {
			next = maxSprites
		}
		a.logic.SetSpriteCount(next)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		stats := a.logic.Stats()
		next := stats.SpriteCount - spriteStep
		if next < minSprites {
			next = minSprites
		}
		a.logic.SetSpriteCount(next)
	}

	axisX, hasGamepad := readPrimaryGamepadAxisX()
	a.lastAxisX = axisX
	a.hasGamepad = hasGamepad

	a.logic.Update(1.0/60.0, axisX)
	a.updateMSAvg = ema(a.updateMSAvg, float64(time.Since(start).Microseconds())/1000.0, 0.12)
	return nil
}

func (a *app) Draw(screen *ebiten.Image) {
	start := time.Now()

	screen.Fill(color.RGBA{R: 12, G: 14, B: 24, A: 255})

	half := a.logic.Width() * 0.5
	screen.DrawImage(a.leftOverlay, nil)
	rightOpts := &ebiten.DrawImageOptions{}
	rightOpts.GeoM.Translate(half, 0)
	screen.DrawImage(a.rightOverlay, rightOpts)

	a.spriteViews = a.logic.SnapshotInto(a.spriteViews)
	spriteImg, spriteW, spriteH := a.currentSpriteImage()
	a.drawSprites(screen, a.spriteViews, spriteImg, spriteW, spriteH)

	stats := a.logic.Stats()
	progress := a.logic.DirectionProgress()
	bgOpts := &ebiten.DrawImageOptions{}
	bgOpts.GeoM.Translate(float64(a.progressBarX), float64(a.progressBarY))
	screen.DrawImage(a.progressBg, bgOpts)

	fillW := int(float64(a.progressBarW) * progress)
	if fillW > 0 {
		fillOpts := &ebiten.DrawImageOptions{}
		fillSlice := a.progressFillMax.SubImage(image.Rect(0, 0, fillW, 10)).(*ebiten.Image)
		fillOpts.GeoM.Translate(float64(a.progressBarX), float64(a.progressBarY))
		screen.DrawImage(fillSlice, fillOpts)
	}

	spriteMode := "PIXEL"
	if a.useGopher && a.spriteGopher != nil {
		spriteMode = "GOPHER.PNG"
	}

	hud := fmt.Sprintf(
		"spriteFlood\nCurrent target: %s\nScore: %d | Misses: %d\nSprites: %d\nSprite mode: %s\nAnalog X: %.2f\nController connected: %v\nFPS: %.1f | TPS: %.1f\nUpdate(ms): %.3f | Draw(ms): %.3f\n\nControls:\n- Left analog: pull sprites\n- Up Arrow: +10k sprites (max 100k)\n- Down Arrow: -10k sprites (min 10k)\n- G key: toggle Pixel/Gopher\n\nGoal:\nKeep sprites on the indicated side (LEFT/RIGHT).",
		stats.TargetDirection,
		stats.Score,
		stats.Missed,
		stats.SpriteCount,
		spriteMode,
		a.lastAxisX,
		a.hasGamepad,
		ebiten.ActualFPS(),
		ebiten.ActualTPS(),
		a.updateMSAvg,
		a.drawMSAvg,
	)
	ebitenutil.DebugPrintAt(screen, hud, 20, 36)

	a.drawMSAvg = ema(a.drawMSAvg, float64(time.Since(start).Microseconds())/1000.0, 0.12)
}

func (a *app) Layout(_, _ int) (int, int) {
	return int(a.logic.Width()), int(a.logic.Height())
}

func readPrimaryGamepadAxisX() (float64, bool) {
	ids := ebiten.AppendGamepadIDs(nil)
	if len(ids) == 0 {
		return 0, false
	}
	axis := ebiten.GamepadAxisValue(ids[0], 0)
	return float64(axis), true
}

func (a *app) currentSpriteImage() (*ebiten.Image, float64, float64) {
	if a.useGopher && a.spriteGopher != nil {
		b := a.spriteGopher.Bounds()
		return a.spriteGopher, float64(b.Dx()), float64(b.Dy())
	}
	b := a.spritePixel.Bounds()
	return a.spritePixel, float64(b.Dx()), float64(b.Dy())
}

func loadGopherImage(path string) *ebiten.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return ebiten.NewImageFromImage(img)
}

func ema(prev, sample, alpha float64) float64 {
	if prev == 0 {
		return sample
	}
	return prev*(1.0-alpha) + sample*alpha
}

func makeTriangleIndices(spriteCapacity int) []uint16 {
	indices := make([]uint16, spriteCapacity*6)
	for i := 0; i < spriteCapacity; i++ {
		vi := uint16(i * 4)
		ii := i * 6
		indices[ii] = vi
		indices[ii+1] = vi + 1
		indices[ii+2] = vi + 2
		indices[ii+3] = vi
		indices[ii+4] = vi + 2
		indices[ii+5] = vi + 3
	}
	return indices
}

func (a *app) drawSprites(screen *ebiten.Image, sprites []core.SpriteView, src *ebiten.Image, srcW, srcH float64) {
	b := src.Bounds()
	sx0 := float32(b.Min.X)
	sy0 := float32(b.Min.Y)
	sx1 := float32(b.Max.X)
	sy1 := float32(b.Max.Y)

	maxDim := srcW
	if srcH > maxDim {
		maxDim = srcH
	}
	if maxDim <= 0 {
		maxDim = 1
	}

	const inv255 = 1.0 / 255.0

	for start := 0; start < len(sprites); start += maxSpritesPerTriangleBatch {
		end := start + maxSpritesPerTriangleBatch
		if end > len(sprites) {
			end = len(sprites)
		}
		batch := sprites[start:end]

		for i, s := range batch {
			target := s.Size * s.Scale
			scale := target / maxDim
			halfW := float32(srcW * scale * 0.5)
			halfH := float32(srcH * scale * 0.5)

			sn, cs := math.Sincos(s.Angle)
			sinA := float32(sn)
			cosA := float32(cs)
			cx := float32(s.X)
			cy := float32(s.Y)

			r := float32(s.Tint.R) * inv255
			g := float32(s.Tint.G) * inv255
			bl := float32(s.Tint.B) * inv255
			al := float32(s.Tint.A) * inv255

			vi := i * 4
			fillVertex := func(off int, lx, ly, sx, sy float32) {
				rx := lx*cosA - ly*sinA
				ry := lx*sinA + ly*cosA
				a.triVertices[vi+off] = ebiten.Vertex{
					DstX:   cx + rx,
					DstY:   cy + ry,
					SrcX:   sx,
					SrcY:   sy,
					ColorR: r,
					ColorG: g,
					ColorB: bl,
					ColorA: al,
				}
			}

			fillVertex(0, -halfW, -halfH, sx0, sy0)
			fillVertex(1, halfW, -halfH, sx1, sy0)
			fillVertex(2, halfW, halfH, sx1, sy1)
			fillVertex(3, -halfW, halfH, sx0, sy1)
		}

		vcount := len(batch) * 4
		icount := len(batch) * 6
		screen.DrawTriangles(a.triVertices[:vcount], a.triIndices[:icount], src, nil)
	}
}
