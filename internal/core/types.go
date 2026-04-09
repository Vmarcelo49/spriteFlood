package core

import "image/color"

type Side int

const (
	SideLeft Side = iota
	SideRight
)

func (s Side) String() string {
	if s == SideLeft {
		return "ESQUERDA"
	}
	return "DIREITA"
}

type Sprite struct {
	X        float64
	Y        float64
	FallVel  float64
	Angle    float64
	Angular  float64
	Scale    float64
	ScaleVel float64
	Hue      float64
}

type SpriteView struct {
	X     float64
	Y     float64
	Angle float64
	Scale float64
	Size  float64
	Tint  color.RGBA
}

type Stats struct {
	Score           int64
	Missed          int64
	TargetDirection Side
	SpriteCount     int
}
