package config

import "time"

const (
	// Network configuration
	DISCOVERY_PORT = 9999
	API_PORT       = "10000"

	// Announcement settings
	BROADCAST_INTERVAL = 5 * time.Second
	DEVICE_TIMEOUT     = 15 * time.Second
	SYNCD_DISCOVER     = "SYNCD_DISCOVER"
)
