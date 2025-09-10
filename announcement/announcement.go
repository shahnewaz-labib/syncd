package announcement

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"syncd/utils"
)

const (
	BROADCAST_INTERVAL = 5 * time.Second
	SYNCD_DISCOVER     = "SYNCD_DISCOVER"
	DEVICE_TIMEOUT     = 15 * time.Second
)

type Device struct {
	DeviceID string
	Username string
	IP       string
	LastSeen time.Time
}

var (
	onlineDevices = make(map[string]*Device)
	devicesMutex  sync.RWMutex
)

type Announcement struct {
	DeviceID string
	Username string
	Type     string
}

func New(deviceID string, username string) Announcement {
	return Announcement{
		DeviceID: deviceID,
		Username: username,
		Type:     SYNCD_DISCOVER,
	}
}

func (a Announcement) ToBytes() []byte {
	message := fmt.Sprintf("%s|%s|%s", a.Type, a.DeviceID, a.Username)
	return []byte(message)
}

func (a Announcement) String() string {
	return a.Type
}

const PORT = 9999

func Broadcast() {
	conn, err := net.Dial("udp", "255.255.255.255:"+fmt.Sprint(PORT))
	if err != nil {
		fmt.Printf("Broadcast error: %v\n", err)
		return
	}
	defer conn.Close()

	deviceInfo, err := utils.GetDeviceInfo()
	if err != nil {
		fmt.Printf("Error getting device info: %v\n", err)
		return
	}
	announcement := New(deviceInfo.UniqueDeviceID, utils.GetUsername())
	for {
		conn.Write(announcement.ToBytes())
		time.Sleep(BROADCAST_INTERVAL)
	}
}

func Listen() {
	addr, err := net.ResolveUDPAddr("udp", ":"+fmt.Sprint(PORT))
	if err != nil {
		fmt.Printf("Address error: %v\n", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Listen error: %v\n", err)
		return
	}
	defer conn.Close()

	go cleanupOfflineDevices()

	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			continue
		}
		message := string(buf[:n])
		parts := strings.Split(message, "|")

		if len(parts) >= 3 && parts[0] == SYNCD_DISCOVER {
			deviceID := parts[1]
			username := parts[2]
			ip := clientAddr.IP.String()

			updateOnlineDevice(deviceID, username, ip)
		}
	}
}

func updateOnlineDevice(deviceID, username, ip string) {
	devicesMutex.Lock()
	defer devicesMutex.Unlock()

	device := &Device{
		DeviceID: deviceID,
		Username: username,
		IP:       ip,
		LastSeen: time.Now(),
	}

	onlineDevices[deviceID] = device
	fmt.Printf("Device online: %s (%s) - %s\n", username, deviceID, ip)
}

func cleanupOfflineDevices() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		devicesMutex.Lock()
		now := time.Now()

		for deviceID, device := range onlineDevices {
			if now.Sub(device.LastSeen) > DEVICE_TIMEOUT {
				fmt.Printf("Device offline: %s (%s)\n", device.Username, device.DeviceID)
				delete(onlineDevices, deviceID)
			}
		}

		devicesMutex.Unlock()
	}
}

func GetOnlineDevices() []*Device {
	devicesMutex.RLock()
	defer devicesMutex.RUnlock()

	devices := make([]*Device, 0, len(onlineDevices))
	for _, device := range onlineDevices {
		devices = append(devices, device)
	}

	return devices
}
