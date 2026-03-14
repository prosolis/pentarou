package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to config.yml")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("FATAL: %v", err)
		os.Exit(1)
	}

	if err := RunServer(cfg); err != nil {
		log.Printf("FATAL: %v", err)
		os.Exit(1)
	}
}
