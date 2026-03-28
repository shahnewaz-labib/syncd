package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	node1IP = "172.28.0.10"
	node2IP = "172.28.0.11"
	apiPort = "10000"
)

func TestMain(m *testing.M) {
	// Setup: start containers
	if err := setupContainers(); err != nil {
		fmt.Printf("Failed to setup containers: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown
	teardownContainers()

	os.Exit(code)
}

func setupContainers() error {
	// Create test file
	testDir := filepath.Join(".", "testfiles")
	os.MkdirAll(testDir, 0755)

	testFile := filepath.Join(testDir, "test.txt")
	testContent := []byte("Hello from syncd integration test!\nThis is a test file.\n")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	// Create larger test file (1MB)
	largeFile := filepath.Join(testDir, "large.bin")
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		return fmt.Errorf("failed to create large test file: %w", err)
	}

	// Build and start containers
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.test.yml", "up", "-d", "--build")
	cmd.Dir = ".."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	// Wait for services to be ready
	fmt.Println("Waiting for services to start...")
	time.Sleep(5 * time.Second)

	// Wait for API to be accessible
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s:%s/devices", node1IP, apiPort))
		if err == nil {
			resp.Body.Close()
			fmt.Println("Services are ready!")
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("services did not become ready in time")
}

func teardownContainers() {
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.test.yml", "down", "-v")
	cmd.Dir = ".."
	cmd.Run()
}

func TestDeviceDiscovery(t *testing.T) {
	// Wait for discovery
	time.Sleep(3 * time.Second)

	// Check node1 can see node2
	resp, err := http.Get(fmt.Sprintf("http://%s:%s/devices", node1IP, apiPort))
	if err != nil {
		t.Fatalf("Failed to get devices from node1: %v", err)
	}
	defer resp.Body.Close()

	var devices []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should see at least node2
	found := false
	for _, d := range devices {
		if ip, ok := d["ip"].(string); ok && ip == node2IP {
			found = true
			t.Logf("Node1 discovered Node2: %v", d)
			break
		}
	}

	if !found {
		t.Errorf("Node1 did not discover Node2. Devices: %v", devices)
	}
}

func TestFileTransfer(t *testing.T) {
	// Wait for discovery
	time.Sleep(3 * time.Second)

	// Send file from node1 to node2
	testFile := "/testfiles/test.txt"

	// Execute send command on node1
	cmd := exec.Command("docker", "exec", "syncd-node1",
		"syncd", "send", testFile, "--to", node2IP, "--wait")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Logf("Send stdout: %s", stdout.String())
	t.Logf("Send stderr: %s", stderr.String())

	if err != nil {
		t.Fatalf("Failed to send file: %v", err)
	}

	// Verify file was received
	// Note: This requires the receiver to auto-accept, which we need to implement
	// For now, check that the transfer was initiated
	if !strings.Contains(stdout.String(), "Sending") {
		t.Errorf("Expected 'Sending' in output, got: %s", stdout.String())
	}
}

func TestLargeFileTransfer(t *testing.T) {
	t.Skip("Requires auto-accept feature for headless transfer")

	// This test would verify large file transfer with checksum
	testFile := "/testfiles/large.bin"

	// Calculate original checksum
	originalData, err := os.ReadFile(filepath.Join("testfiles", "large.bin"))
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}
	originalHash := sha256.Sum256(originalData)

	// Send file
	cmd := exec.Command("docker", "exec", "syncd-node1",
		"syncd", "send", testFile, "--to", node2IP, "--wait")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to send large file: %v", err)
	}

	// Read received file and verify checksum
	receivedCmd := exec.Command("docker", "exec", "syncd-node2",
		"cat", "/downloads/large.bin")
	receivedData, err := receivedCmd.Output()
	if err != nil {
		t.Fatalf("Failed to read received file: %v", err)
	}

	receivedHash := sha256.Sum256(receivedData)
	if originalHash != receivedHash {
		t.Errorf("Checksum mismatch: original=%x, received=%x", originalHash, receivedHash)
	}
}

func TestAPIEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		wantCode int
	}{
		{"Get devices", "/devices", "GET", 200},
		{"Transfer status not found", "/transfer/status/nonexistent", "GET", 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("http://%s:%s%s", node1IP, apiPort, tt.endpoint)

			req, err := http.NewRequest(tt.method, url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Got status %d, want %d. Body: %s", resp.StatusCode, tt.wantCode, body)
			}
		})
	}
}

func TestTransferRequest(t *testing.T) {
	// Test sending a transfer request via API
	url := fmt.Sprintf("http://%s:%s/transfer/request", node2IP, apiPort)

	payload := `{
		"transfer_id": "test-123",
		"sender_id": "sender-device-id",
		"sender_name": "alice",
		"sender_ip": "172.28.0.10",
		"file_name": "test.txt",
		"file_size": 1024,
		"checksum": "abc123"
	}`

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed to send transfer request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 202 Accepted, got %d: %s", resp.StatusCode, body)
	}
}
