package announcement

import (
	"fmt"
	"net"
	"time"

	"syncd/utils"
)

const (
	BROADCAST_INTERVAL = 5 * time.Second
	SYNCD_DISCOVER     = "SYNCD_DISCOVER"
)

type Announcement struct {
	DeviceID string
	Type     string
}

func New(deviceID string) Announcement {
	return Announcement{
		DeviceID: deviceID,
		Type:     SYNCD_DISCOVER,
	}
}

func (a Announcement) ToBytes() []byte {
	return []byte(a.Type)
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
	announcement := New(deviceInfo.UniqueDeviceID)
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

	myIP := utils.GetLocalIP()
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			continue
		}
		if string(buf[:n]) == SYNCD_DISCOVER {
			if clientAddr.IP.String() == myIP {
				fmt.Printf("Player found: %s (self)\n", clientAddr.IP)
			} else {
				fmt.Printf("Player found: %s\n", clientAddr.IP)
			}
		}
	}
}
