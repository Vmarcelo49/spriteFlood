//go:build raylib

package main

import (
	"log"

	"spriteflood/internal/core"
)

func main() {
	cfg := core.DefaultConfig()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}
