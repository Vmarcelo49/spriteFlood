package core

import (
	"image/color"
	"math"
	"math/rand"
)

type Game struct {
	cfg               Config
	sprites           []Sprite
	targetDirection   Side
	directionCooldown float64
	rng               *rand.Rand
	score             int64
	missed            int64
}

func NewGame(cfg Config) *Game {
	rng := rand.New(rand.NewSource(cfg.RNGSeed))
	g := &Game{
		cfg:             cfg,
		sprites:         make([]Sprite, cfg.SpriteCount),
		targetDirection: SideLeft,
		rng:             rng,
	}
	g.directionCooldown = cfg.DirectionInterval.Seconds()
	for i := range g.sprites {
		g.respawn(i, true)
	}
	return g
}

func (g *Game) Update(dt float64, axisX float64) {
	if dt <= 0 {
		return
	}

	g.directionCooldown -= dt
	if g.directionCooldown <= 0 {
		if g.targetDirection == SideLeft {
			g.targetDirection = SideRight
		} else {
			g.targetDirection = SideLeft
		}
		g.directionCooldown += g.cfg.DirectionInterval.Seconds()
	}

	axis := clamp(axisX, -1, 1)
	if math.Abs(axis) < 0.18 {
		axis = 0
	}

	for i := range g.sprites {
		s := &g.sprites[i]

		passive := sideSign(g.targetDirection) * g.cfg.PassiveDrift
		s.X += (axis*g.cfg.HorizontalControl + passive) * dt
		s.Y += s.FallVel * dt
		s.Angle += s.Angular * dt
		s.Scale += s.ScaleVel * dt
		s.Hue += 95 * dt
		if s.Hue >= 360 {
			s.Hue -= 360
		}

		if s.Scale < g.cfg.MinScale {
			s.Scale = g.cfg.MinScale
			s.ScaleVel *= -1
		}
		if s.Scale > g.cfg.MaxScale {
			s.Scale = g.cfg.MaxScale
			s.ScaleVel *= -1
		}

		if s.X < -g.cfg.ScreenWidth*0.2 {
			s.X = g.cfg.ScreenWidth * 1.2
		}
		if s.X > g.cfg.ScreenWidth*1.2 {
			s.X = -g.cfg.ScreenWidth * 0.2
		}

		if s.Y > g.cfg.ScreenHeight+g.cfg.SpriteSize*3 {
			correct := (g.targetDirection == SideLeft && s.X <= g.cfg.ScreenWidth*0.5) ||
				(g.targetDirection == SideRight && s.X > g.cfg.ScreenWidth*0.5)
			if correct {
				g.score++
			} else {
				g.missed++
			}
			g.respawn(i, false)
		}
	}
}

func (g *Game) Snapshot() []SpriteView {
	return g.SnapshotInto(nil)
}

func (g *Game) SnapshotInto(dst []SpriteView) []SpriteView {
	if cap(dst) < len(g.sprites) {
		dst = make([]SpriteView, len(g.sprites))
	} else {
		dst = dst[:len(g.sprites)]
	}

	out := dst
	for i, s := range g.sprites {
		out[i] = SpriteView{
			X:     s.X,
			Y:     s.Y,
			Angle: s.Angle,
			Scale: s.Scale,
			Size:  g.cfg.SpriteSize,
			Tint:  hueToRGBA(s.Hue),
		}
	}
	return out
}

func (g *Game) Stats() Stats {
	return Stats{
		Score:           g.score,
		Missed:          g.missed,
		TargetDirection: g.targetDirection,
		SpriteCount:     len(g.sprites),
	}
}

func (g *Game) DirectionProgress() float64 {
	total := g.cfg.DirectionInterval.Seconds()
	if total <= 0 {
		return 0
	}
	remaining := clamp(g.directionCooldown, 0, total)
	return remaining / total
}

func (g *Game) Width() float64 {
	return g.cfg.ScreenWidth
}

func (g *Game) Height() float64 {
	return g.cfg.ScreenHeight
}

func (g *Game) SetSpriteCount(count int) int {
	if count < 1 {
		count = 1
	}

	current := len(g.sprites)
	if count == current {
		return current
	}

	if count < current {
		g.sprites = g.sprites[:count]
		return count
	}

	grow := count - current
	g.sprites = append(g.sprites, make([]Sprite, grow)...)
	for i := current; i < len(g.sprites); i++ {
		g.respawn(i, true)
	}
	return len(g.sprites)
}

func (g *Game) respawn(i int, spreadFromTop bool) {
	s := &g.sprites[i]
	s.X = g.rng.Float64() * g.cfg.ScreenWidth
	if spreadFromTop {
		s.Y = -g.rng.Float64() * g.cfg.ScreenHeight
	} else {
		s.Y = -g.cfg.SpriteSize * (1 + g.rng.Float64()*10)
	}
	s.FallVel = lerp(g.cfg.MinFallSpeed, g.cfg.MaxFallSpeed, g.rng.Float64())
	s.Angle = g.rng.Float64() * 2 * math.Pi
	s.Angular = lerp(g.cfg.MinAngularVelocity, g.cfg.MaxAngularVelocity, g.rng.Float64())
	s.Scale = lerp(g.cfg.MinScale, g.cfg.MaxScale, g.rng.Float64())
	s.ScaleVel = lerp(g.cfg.MinScaleVel, g.cfg.MaxScaleVel, g.rng.Float64())
	if g.rng.Intn(2) == 0 {
		s.ScaleVel *= -1
	}
	s.Hue = g.rng.Float64() * 360
}

func sideSign(side Side) float64 {
	if side == SideLeft {
		return -1
	}
	return 1
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func hueToRGBA(h float64) color.RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := 1.0
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))

	r, g, b := 0.0, 0.0, 0.0
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}

	toByte := func(v float64) uint8 {
		if v <= 0 {
			return 0
		}
		if v >= 1 {
			return 255
		}
		return uint8(v * 255)
	}

	return color.RGBA{R: toByte(r), G: toByte(g), B: toByte(b), A: 255}
}
