package main

import (
	"flag"
	"fmt"

	"syncd/config"
	discover "syncd/discover"
)

type Config struct {
	Name  string
	Debug bool
}

func parseFlags() (*config.Config, error) {
	name := flag.String("name", "", "Name of the device")
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	if *name == "" {
		return nil, fmt.Errorf("--name flag is required")
	}

	return &config.Config{
		Name:  *name,
		Debug: *debug,
	}, nil
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	go discover.StartAnnouncementService(cfg)
	go discover.Listen()

	select {}
}
