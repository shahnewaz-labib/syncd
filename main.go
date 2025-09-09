package main

import (
	"time"

	"syncd/announcement"
	"syncd/utils"
)

const (
	PORT = 9999
)

func main() {
	utils.PrintSysInfo()
	go announcement.Listen()
	time.Sleep(100 * time.Millisecond)
	go announcement.Broadcast()
	select {} // Keep running
}
