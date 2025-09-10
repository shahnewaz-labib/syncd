package main

import (
	"fmt"
	"time"

	"syncd/announcement"
	"syncd/api"
	"syncd/config"
	"syncd/utils"
)

func main() {
	utils.PrintSysInfo()
	go announcement.Listen()
	time.Sleep(100 * time.Millisecond)
	go announcement.Broadcast()

	fmt.Printf("Starting API server on port %s\n", config.API_PORT)
	api.StartServer(config.API_PORT)

	select {} // Keep running
}
