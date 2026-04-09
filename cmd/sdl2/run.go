//go:build sdl2

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"time"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/veandco/go-sdl2/sdl"

	"spriteflood/internal/core"
)

const (
	spriteStep                 = 10000
	minSprites                 = 10000
	maxSprites                 = 100000
	maxSpritesPerGeometryBatch = 12000
	hudRefreshInterval         = 120 * time.Millisecond

	hudWidth  = 900
	hudHeight = 430
)

type keyLatch struct {
	up   bool
	down bool
	g    bool
}

func run(cfg core.Config) error {
	_ = sdl.SetHint(sdl.HINT_RENDER_BATCHING, "1")
	_ = sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0")

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_GAMECONTROLLER | sdl.INIT_EVENTS); err != nil {
		return fmt.Errorf("failed to initialize SDL2: %w", err)
	}
	defer sdl.Quit()

	window, err := sdl.CreateWindow(
		"spriteFlood - SDL2",
		sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED,
		int32(cfg.ScreenWidth),
		int32(cfg.ScreenHeight),
		sdl.WINDOW_SHOWN,
	)
	if err != nil {
		return fmt.Errorf("failed to create SDL2 window: %w", err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_SOFTWARE)
		if err != nil {
			return fmt.Errorf("failed to create SDL2 renderer: %w", err)
		}
	}
	defer renderer.Destroy()
	_ = renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)

	spritePixel, err := createSolidTexture(renderer, 1, 1, color.RGBA{255, 255, 255, 255})
	if err != nil {
		return err
	}
	defer spritePixel.Destroy()

	gopherTexture, gopherW, gopherH, hasGopher := loadTextureFromImage(renderer, "gopher.png")
	if hasGopher {
		defer gopherTexture.Destroy()
	}
	useGopher := hasGopher

	hudTexture, err := renderer.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STREAMING, hudWidth, hudHeight)
	if err != nil {
		return fmt.Errorf("failed to create HUD texture: %w", err)
	}
	defer hudTexture.Destroy()
	_ = hudTexture.SetBlendMode(sdl.BLENDMODE_BLEND)
	hudCanvas := image.NewRGBA(image.Rect(0, 0, hudWidth, hudHeight))
	hudNextUpdate := time.Now()

	game := core.NewGame(cfg)
	spriteViews := make([]core.SpriteView, 0, cfg.SpriteCount)
	geomVertices := make([]sdl.Vertex, maxSpritesPerGeometryBatch*4)
	geomIndices := makeGeometryIndices(maxSpritesPerGeometryBatch)

	controller := openFirstController()
	if controller != nil {
		defer controller.Close()
	}

	var keys keyLatch
	var updateMSAvg float64
	var drawMSAvg float64
	var lastAxisX float64
	var hasGamepad bool
	var fpsApprox float64
	var upsApprox float64
	perfWindowStart := time.Now()
	renderedFrames := 0
	updatedFrames := 0
	lastTick := time.Now()

	running := true
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				running = false
			case *sdl.ControllerDeviceEvent:
				if t.Type == sdl.CONTROLLERDEVICEADDED {
					if controller == nil {
						idx := int(t.Which)
						if sdl.IsGameController(idx) {
							controller = sdl.GameControllerOpen(idx)
						}
					}
				}
				if t.Type == sdl.CONTROLLERDEVICEREMOVED {
					if controller != nil {
						jid := controller.Joystick().InstanceID()
						if jid == t.Which {
							controller.Close()
							controller = nil
						}
					}
				}
			}
		}

		updateStart := time.Now()
		now := time.Now()
		dt := now.Sub(lastTick).Seconds()
		lastTick = now
		if dt <= 0 {
			dt = 1.0 / 60.0
		}
		if dt > 0.05 {
			dt = 0.05
		}

		state := sdl.GetKeyboardState()
		upNow := state[sdl.SCANCODE_UP] != 0
		downNow := state[sdl.SCANCODE_DOWN] != 0
		gNow := state[sdl.SCANCODE_G] != 0

		if upNow && !keys.up {
			next := game.Stats().SpriteCount + spriteStep
			if next > maxSprites {
				next = maxSprites
			}
			game.SetSpriteCount(next)
			hudNextUpdate = time.Time{}
		}
		if downNow && !keys.down {
			next := game.Stats().SpriteCount - spriteStep
			if next < minSprites {
				next = minSprites
			}
			game.SetSpriteCount(next)
			hudNextUpdate = time.Time{}
		}
		if gNow && !keys.g {
			if hasGopher {
				useGopher = !useGopher
				hudNextUpdate = time.Time{}
			}
		}
		keys.up = upNow
		keys.down = downNow
		keys.g = gNow

		axisX, connected := readAxisX(controller)
		hasGamepad = connected
		lastAxisX = axisX
		game.Update(dt, axisX)
		updatedFrames++
		updateMSAvg = ema(updateMSAvg, float64(time.Since(updateStart).Microseconds())/1000.0, 0.12)

		drawStart := time.Now()
		_ = renderer.SetDrawColor(12, 14, 24, 255)
		_ = renderer.Clear()

		halfW := int32(cfg.ScreenWidth * 0.5)
		_ = renderer.SetDrawColor(20, 70, 80, 24)
		_ = renderer.FillRect(&sdl.Rect{X: 0, Y: 0, W: halfW, H: int32(cfg.ScreenHeight)})
		_ = renderer.SetDrawColor(80, 30, 30, 24)
		_ = renderer.FillRect(&sdl.Rect{X: halfW, Y: 0, W: int32(cfg.ScreenWidth) - halfW, H: int32(cfg.ScreenHeight)})

		spriteViews = game.SnapshotInto(spriteViews)
		if useGopher && hasGopher {
			drawSpritesTexture(renderer, spriteViews, gopherTexture, gopherW, gopherH, geomVertices, geomIndices)
		} else {
			drawSpritesGeometry(renderer, spriteViews, geomVertices, geomIndices)
		}

		stats := game.Stats()
		directionProgress := game.DirectionProgress()
		drawProgressBar(renderer, int32(cfg.ScreenWidth), directionProgress)

		if time.Now().After(hudNextUpdate) {
			hudLines := buildHUDLines(stats, useGopher, hasGopher, hasGamepad, lastAxisX, fpsApprox, upsApprox, updateMSAvg, drawMSAvg)
			if err := drawHUD(hudTexture, hudCanvas, hudLines); err == nil {
				hudNextUpdate = time.Now().Add(hudRefreshInterval)
			}
		}
		_ = renderer.Copy(hudTexture, nil, &sdl.Rect{X: 20, Y: 36, W: hudWidth, H: hudHeight})

		renderer.Present()
		renderedFrames++
		drawMSAvg = ema(drawMSAvg, float64(time.Since(drawStart).Microseconds())/1000.0, 0.12)

		elapsed := time.Since(perfWindowStart)
		if elapsed >= time.Second {
			seconds := elapsed.Seconds()
			fpsApprox = float64(renderedFrames) / seconds
			upsApprox = float64(updatedFrames) / seconds
			renderedFrames = 0
			updatedFrames = 0
			perfWindowStart = time.Now()
		}
	}

	return nil
}

func buildHUDLines(stats core.Stats, useGopher bool, hasGopher bool, hasGamepad bool, axisX float64, fps float64, ups float64, updateMS float64, drawMS float64) []string {
	spriteMode := "PIXEL"
	if useGopher && hasGopher {
		spriteMode = "GOPHER.PNG"
	}
	if !hasGopher {
		spriteMode = "PIXEL (gopher.png not found)"
	}

	return []string{
		"spriteFlood (SDL2)",
		fmt.Sprintf("Current target: %s", stats.TargetDirection),
		fmt.Sprintf("Score: %d | Misses: %d", stats.Score, stats.Missed),
		fmt.Sprintf("Sprites: %d", stats.SpriteCount),
		fmt.Sprintf("Sprite mode: %s", spriteMode),
		fmt.Sprintf("Analog X: %.2f", axisX),
		fmt.Sprintf("Controller connected: %v", hasGamepad),
		fmt.Sprintf("Approx FPS: %.1f | Approx UPS: %.1f", fps, ups),
		fmt.Sprintf("Update(ms): %.3f | Draw(ms): %.3f", updateMS, drawMS),
		"",
		"Controls:",
		"- Left analog: pull sprites",
		"- Up Arrow: +10k sprites (max 100k)",
		"- Down Arrow: -10k sprites (min 10k)",
		"- G key: toggle Pixel/Gopher",
	}
}

func drawHUD(tex *sdl.Texture, img *image.RGBA, lines []string) error {
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 0}), image.Point{}, draw.Src)

	y := 14
	for _, line := range lines {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.RGBA{238, 244, 255, 255}),
			Face: basicfont.Face7x13,
			Dot:  fixed.P(8, y),
		}
		d.DrawString(line)
		y += 24
	}

	if img == nil || len(img.Pix) == 0 {
		return nil
	}
	return tex.Update(nil, unsafe.Pointer(&img.Pix[0]), img.Stride)
}

func drawProgressBar(renderer *sdl.Renderer, screenWidth int32, progress float64) {
	barW := int32(float64(screenWidth) * 0.8)
	barX := (screenWidth - barW) / 2
	barY := int32(16)
	barH := int32(10)
	_ = renderer.SetDrawColor(44, 44, 60, 255)
	_ = renderer.FillRect(&sdl.Rect{X: barX, Y: barY, W: barW, H: barH})
	fillW := int32(float64(barW) * progress)
	if fillW > 0 {
		_ = renderer.SetDrawColor(250, 200, 90, 255)
		_ = renderer.FillRect(&sdl.Rect{X: barX, Y: barY, W: fillW, H: barH})
	}
}

func drawSpritesTexture(renderer *sdl.Renderer, sprites []core.SpriteView, texture *sdl.Texture, baseW int32, baseH int32, vertices []sdl.Vertex, indices []int32) {
	maxDim := float64(baseW)
	if float64(baseH) > maxDim {
		maxDim = float64(baseH)
	}
	if maxDim <= 0 {
		maxDim = 1
	}

	for start := 0; start < len(sprites); start += maxSpritesPerGeometryBatch {
		end := start + maxSpritesPerGeometryBatch
		if end > len(sprites) {
			end = len(sprites)
		}
		batch := sprites[start:end]

		for i, s := range batch {
			target := s.Size * s.Scale
			scale := target / maxDim
			halfW := float32(float64(baseW) * scale * 0.5)
			halfH := float32(float64(baseH) * scale * 0.5)

			sn, cs := math.Sincos(s.Angle)
			sinA := float32(sn)
			cosA := float32(cs)
			cx := float32(s.X)
			cy := float32(s.Y)

			vi := i * 4
			fillVertex := func(off int, lx, ly, tx, ty float32) {
				rx := lx*cosA - ly*sinA
				ry := lx*sinA + ly*cosA
				vertices[vi+off] = sdl.Vertex{
					Position: sdl.FPoint{X: cx + rx, Y: cy + ry},
					Color:    sdl.Color{R: s.Tint.R, G: s.Tint.G, B: s.Tint.B, A: s.Tint.A},
					TexCoord: sdl.FPoint{X: tx, Y: ty},
				}
			}

			fillVertex(0, -halfW, -halfH, 0, 0)
			fillVertex(1, halfW, -halfH, 1, 0)
			fillVertex(2, halfW, halfH, 1, 1)
			fillVertex(3, -halfW, halfH, 0, 1)
		}

		vcount := len(batch) * 4
		icount := len(batch) * 6
		_ = renderer.RenderGeometry(texture, vertices[:vcount], indices[:icount])
	}
}

func drawSpritesGeometry(renderer *sdl.Renderer, sprites []core.SpriteView, vertices []sdl.Vertex, indices []int32) {
	for start := 0; start < len(sprites); start += maxSpritesPerGeometryBatch {
		end := start + maxSpritesPerGeometryBatch
		if end > len(sprites) {
			end = len(sprites)
		}
		batch := sprites[start:end]

		for i, s := range batch {
			half := float32(s.Size * s.Scale * 0.5)
			if half < 0.5 {
				half = 0.5
			}

			cx := float32(s.X)
			cy := float32(s.Y)
			cosA := float32(math.Cos(s.Angle))
			sinA := float32(math.Sin(s.Angle))

			local := [4]sdl.FPoint{{X: -half, Y: -half}, {X: half, Y: -half}, {X: half, Y: half}, {X: -half, Y: half}}

			vi := i * 4
			for i, p := range local {
				rx := p.X*cosA - p.Y*sinA
				ry := p.X*sinA + p.Y*cosA
				vertices[vi+i] = sdl.Vertex{
					Position: sdl.FPoint{X: cx + rx, Y: cy + ry},
					Color:    sdl.Color{R: s.Tint.R, G: s.Tint.G, B: s.Tint.B, A: s.Tint.A},
					TexCoord: sdl.FPoint{},
				}
			}
		}

		vcount := len(batch) * 4
		icount := len(batch) * 6
		_ = renderer.RenderGeometry(nil, vertices[:vcount], indices[:icount])
	}
}

func makeGeometryIndices(spriteCapacity int) []int32 {
	indices := make([]int32, spriteCapacity*6)
	for i := 0; i < spriteCapacity; i++ {
		vi := int32(i * 4)
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

func readAxisX(controller *sdl.GameController) (float64, bool) {
	if controller == nil || !controller.Attached() {
		return 0, false
	}
	v := float64(controller.Axis(sdl.CONTROLLER_AXIS_LEFTX)) / 32767.0
	if math.Abs(v) < 0.18 {
		v = 0
	}
	if v < -1 {
		v = -1
	}
	if v > 1 {
		v = 1
	}
	return v, true
}

func openFirstController() *sdl.GameController {
	count := sdl.NumJoysticks()
	for i := 0; i < count; i++ {
		if !sdl.IsGameController(i) {
			continue
		}
		c := sdl.GameControllerOpen(i)
		if c != nil {
			return c
		}
	}
	return nil
}

func createSolidTexture(renderer *sdl.Renderer, w, h int32, c color.RGBA) (*sdl.Texture, error) {
	tex, err := renderer.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STREAMING, w, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create base texture: %w", err)
	}
	_ = tex.SetBlendMode(sdl.BLENDMODE_BLEND)

	pix := make([]byte, int(w*h*4))
	for i := 0; i < len(pix); i += 4 {
		pix[i] = c.R
		pix[i+1] = c.G
		pix[i+2] = c.B
		pix[i+3] = c.A
	}
	if err := tex.Update(nil, unsafe.Pointer(&pix[0]), int(w*4)); err != nil {
		tex.Destroy()
		return nil, fmt.Errorf("failed to update base texture: %w", err)
	}
	return tex, nil
}

func loadTextureFromImage(renderer *sdl.Renderer, path string) (*sdl.Texture, int32, int32, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, false
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, false
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)

	tex, err := renderer.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STREAMING, int32(w), int32(h))
	if err != nil {
		return nil, 0, 0, false
	}
	_ = tex.SetBlendMode(sdl.BLENDMODE_BLEND)
	if len(rgba.Pix) == 0 {
		tex.Destroy()
		return nil, 0, 0, false
	}
	if err := tex.Update(nil, unsafe.Pointer(&rgba.Pix[0]), rgba.Stride); err != nil {
		tex.Destroy()
		return nil, 0, 0, false
	}
	return tex, int32(w), int32(h), true
}

func ema(prev, sample, alpha float64) float64 {
	if prev == 0 {
		return sample
	}
	return prev*(1.0-alpha) + sample*alpha
}
