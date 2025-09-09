package utils

import (
	"fmt"
	"net"
	"runtime"
	"strings"
)

func PrintSysInfo() {
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("ARCH: %s\n", runtime.GOARCH)
	fmt.Printf("IP: %s\n", GetLocalIP())

	deviceinfo, err := GetDeviceInfo()
	if err != nil {
		fmt.Printf("Error getting device info: %v\n", err)
		return
	}
	fmt.Printf("CPU ID: %s\n", deviceinfo.CPUID)
	fmt.Printf("Motherboard Serial: %s\n", deviceinfo.MotherboardSerial)
	fmt.Printf("Unique Device ID: %s\n", deviceinfo.UniqueDeviceID)
}

func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "Unable to determine outbound IP"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func GetLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "Unable to determine local IP"
	}

	var fallbackIP string
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip virtual/VPN interfaces by name patterns
		name := strings.ToLower(iface.Name)
		isVirtual := strings.Contains(name, "warp") ||
			strings.Contains(name, "cloudflare") ||
			strings.Contains(name, "tun") ||
			strings.Contains(name, "tap") ||
			strings.Contains(name, "vpn") ||
			strings.Contains(name, "virtual") ||
			strings.Contains(name, "loopback")

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil { // IPv4
					ip := ipNet.IP.String()

					if !isVirtual {
						// Prefer non-virtual interfaces
						if strings.HasPrefix(ip, "192.168.") ||
							strings.HasPrefix(ip, "10.") ||
							(strings.HasPrefix(ip, "172.") && !strings.HasPrefix(ip, "172.16.")) {
							return ip
						}
					}

					// Keep as fallback if no better option
					if fallbackIP == "" {
						fallbackIP = ip
					}
				}
			}
		}
	}

	if fallbackIP != "" {
		return fallbackIP
	}
	return "No local IP found"
}
