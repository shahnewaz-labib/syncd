package main

import (
	"flag"
	"fmt"
	"os"
	"syncd/cmd/tracker"
)

func main() {
	var role string
	flag.StringVar(&role, "role", "", "Role to run (tracker)")
	flag.Parse()

	if role == "" {
		fmt.Println("Error: --role parameter is required")
		fmt.Println("Usage: syncd --role=tracker")
		os.Exit(1)
	}

	switch role {
	case "tracker":
		tracker.StartTracker()
	case "peer":
		fmt.Println("Peer role is not implemented yet.")
		os.Exit(1)
	default:
		fmt.Printf("Error: unknown role '%s'\n", role)
		fmt.Println("Supported roles: tracker")
		os.Exit(1)
	}
}
