package main

import (
	"syncd/announcement"
	"syncd/api"
	"syncd/utils"
)

func main() {
	utils.PrintSysInfo()

	announcement.ListenAndBroadcast()
	api.StartServer()

	select {} // Keep running
}
