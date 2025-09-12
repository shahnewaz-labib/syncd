package main

import (
	"syncd/announcement"
	"syncd/api"
	"syncd/ui"
	"syncd/utils"
)

func main() {
	utils.PrintSysInfo()

	announcement.ListenAndBroadcast()

	// Start API server (port 10000)
	go api.StartServer()

	// Start UI server (port 10001)
	go ui.StartUIServer()

	select {} // Keep running
}
