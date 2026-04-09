package core

import "time"

// Config centraliza os parametros de simulacao para facilitar tuning entre engines.
type Config struct {
	ScreenWidth        float64
	ScreenHeight       float64
	SpriteCount        int
	SpriteSize         float64
	MinFallSpeed       float64
	MaxFallSpeed       float64
	HorizontalControl  float64
	PassiveDrift       float64
	MinScale           float64
	MaxScale           float64
	MinScaleVel        float64
	MaxScaleVel        float64
	MinAngularVelocity float64
	MaxAngularVelocity float64
	DirectionInterval  time.Duration
	RNGSeed            int64
}

func DefaultConfig() Config {
	return Config{
		ScreenWidth:        1280,
		ScreenHeight:       720,
		SpriteCount:        20000,
		SpriteSize:         6,
		MinFallSpeed:       80,
		MaxFallSpeed:       260,
		HorizontalControl:  260,
		PassiveDrift:       120,
		MinScale:           0.4,
		MaxScale:           1.9,
		MinScaleVel:        0.15,
		MaxScaleVel:        1.8,
		MinAngularVelocity: -6.0,
		MaxAngularVelocity: 6.0,
		DirectionInterval:  5 * time.Second,
		RNGSeed:            42,
	}
}
