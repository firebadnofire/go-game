package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	configPath := flag.String("config", "config/game.yml", "path to game configuration")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	game, err := BuildGame(cfg)
	if err != nil {
		log.Fatalf("failed to build game: %v", err)
	}

	ui, err := NewUI(game)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize UI: %v\n", err)
		os.Exit(1)
	}

	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "UI error: %v\n", err)
		os.Exit(1)
	}
}
