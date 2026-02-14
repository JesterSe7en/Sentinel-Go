package main

import (
	"log"
	"os"

	"github.com/JesterSe7en/Sentinel-Go/internal/app"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("failed to load env variables: %s", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %s", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to create app: %s", err)
	}

	if err := application.Run(); err != nil {
		application.Log.Error("server_failed_to_run", "error", err)
		os.Exit(1)
	}
}
