package utils

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type DeviceInfo struct {
	CPUID             string
	MotherboardSerial string
	UniqueDeviceID    string
}

func GetCPUID() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return getCPUIDWindows()
	case "linux":
		return getCPUIDLinux()
	case "darwin":
		return getCPUIDMacOS()
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func GetMotherboardSerial() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return getMotherboardSerialWindows()
	case "linux":
		return getMotherboardSerialLinux()
	case "darwin":
		return getMotherboardSerialMacOS()
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func getCPUIDWindows() (string, error) {
	cmd := exec.Command("wmic", "cpu", "get", "ProcessorId", "/format:value")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CPU ID on Windows: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ProcessorId=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "ProcessorId=")), nil
		}
	}
	return "", fmt.Errorf("CPU ID not found in Windows output")
}

func getMotherboardSerialWindows() (string, error) {
	cmd := exec.Command("wmic", "baseboard", "get", "SerialNumber", "/format:value")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get motherboard serial on Windows: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "SerialNumber=") {
			serial := strings.TrimSpace(strings.TrimPrefix(line, "SerialNumber="))
			if serial != "" && serial != "To be filled by O.E.M." {
				return serial, nil
			}
		}
	}
	return "", fmt.Errorf("motherboard serial not found or invalid")
}

func getCPUIDLinux() (string, error) {
	methods := []func() (string, error){
		func() (string, error) {
			cmd := exec.Command("dmidecode", "-t", "processor")
			output, err := cmd.Output()
			if err != nil {
				return "", err
			}
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), "id:") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						return strings.TrimSpace(parts[1]), nil
					}
				}
			}
			return "", fmt.Errorf("CPU ID not found in dmidecode output")
		},
		func() (string, error) {
			cmd := exec.Command("cat", "/proc/cpuinfo")
			output, err := cmd.Output()
			if err != nil {
				return "", err
			}
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "processor") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						return strings.TrimSpace(parts[1]), nil
					}
				}
			}
			return "", fmt.Errorf("processor info not found")
		},
	}

	for _, method := range methods {
		if result, err := method(); err == nil && result != "" {
			return result, nil
		}
	}

	return "", fmt.Errorf("failed to get CPU ID on Linux")
}

func getMotherboardSerialLinux() (string, error) {
	cmd := exec.Command("dmidecode", "-s", "baseboard-serial-number")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get motherboard serial on Linux: %v", err)
	}

	serial := strings.TrimSpace(string(output))
	if serial != "" && serial != "To be filled by O.E.M." && serial != "Not Specified" {
		return serial, nil
	}

	return "", fmt.Errorf("motherboard serial not found or invalid")
}

func getCPUIDMacOS() (string, error) {
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CPU info on macOS: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func getMotherboardSerialMacOS() (string, error) {
	cmd := exec.Command("system_profiler", "SPHardwareDataType")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get hardware info on macOS: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Serial Number") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("serial number not found in macOS output")
}

func GenerateDeviceID(cpuID, motherboardSerial string) string {
	combined := fmt.Sprintf("%s:%s", cpuID, motherboardSerial)
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

func GetDeviceInfo() (*DeviceInfo, error) {
	cpuID, err := GetCPUID()
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU ID: %v", err)
	}

	motherboardSerial, err := GetMotherboardSerial()
	if err != nil {
		return nil, fmt.Errorf("failed to get motherboard serial: %v", err)
	}

	deviceID := GenerateDeviceID(cpuID, motherboardSerial)

	return &DeviceInfo{
		CPUID:             cpuID,
		MotherboardSerial: motherboardSerial,
		UniqueDeviceID:    deviceID,
	}, nil
}
