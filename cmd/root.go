package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"syncd/announcement"
	"syncd/api"
	"syncd/cli"
	"syncd/events"
	"syncd/transfer"
	"syncd/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	targetFlag     string
	waitFlag       bool
	autoAcceptFlag bool
	savePathFlag   string
)

var rootCmd = &cobra.Command{
	Use:   "syncd",
	Short: "Peer-to-peer file sync and transfer",
	Long:  `syncd is a LAN device discovery and file transfer tool that enables peer-to-peer file sharing between devices on the same network.`,
	Run:   runDashboard,
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run as background daemon (no UI)",
	Long: `Run syncd as a headless daemon for receiving files without the TUI.

Examples:
  syncd daemon
  syncd daemon --auto-accept --save-path /downloads`,
	Run: runDaemon,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show online devices",
	Long:  `Display a list of all devices currently online on the network.`,
	Run:   runStatus,
}

var sendCmd = &cobra.Command{
	Use:   "send <file>",
	Short: "Send a file to a device",
	Long: `Send a file to another device on the network.

Examples:
  syncd send myfile.txt --to 192.168.1.10
  syncd send myfile.txt --to john
  syncd send myfile.txt -t john --wait`,
	Args: cobra.ExactArgs(1),
	Run:  runSend,
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List online devices (script-friendly)",
	Long:  `List online devices in a simple format for scripting.`,
	Run:   runDevices,
}

func init() {
	sendCmd.Flags().StringVarP(&targetFlag, "to", "t", "", "Target device (IP address or username)")
	sendCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "Wait for transfer to complete")
	sendCmd.MarkFlagRequired("to")

	daemonCmd.Flags().BoolVarP(&autoAcceptFlag, "auto-accept", "a", false, "Automatically accept incoming transfers")
	daemonCmd.Flags().StringVarP(&savePathFlag, "save-path", "s", "/downloads", "Directory to save received files")

	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(devicesCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDashboard(cmd *cobra.Command, args []string) {
	// Suppress Gin logging for cleaner TUI
	gin.SetMode(gin.ReleaseMode)

	// Start background services
	announcement.ListenAndBroadcast()

	// Start transfer receiver
	if err := transfer.GetReceiver().StartListener(); err != nil {
		fmt.Printf("Warning: Could not start transfer listener: %v\n", err)
	}

	// Start API server in background
	go api.StartServer()

	// Brief discovery period
	time.Sleep(1 * time.Second)

	// Run dashboard TUI
	if err := cli.RunDashboard(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) {
	fmt.Println("Starting syncd daemon...")
	if autoAcceptFlag {
		fmt.Printf("Auto-accept enabled. Saving files to: %s\n", savePathFlag)
		transfer.SetAutoAccept(true, savePathFlag)
	}

	// Start background services
	announcement.ListenAndBroadcast()

	// Start transfer receiver
	if err := transfer.GetReceiver().StartListener(); err != nil {
		fmt.Printf("Warning: Could not start transfer listener: %v\n", err)
	}

	// Start auto-accept handler if enabled
	if autoAcceptFlag {
		go runAutoAcceptHandler()
	}

	// Start API server (blocking)
	api.StartServer()
}

func runAutoAcceptHandler() {
	eventCh := events.GetEventChannel()
	for event := range eventCh {
		if event.Type == events.TransferRequestReceived {
			payload := event.Payload.(events.TransferRequestPayload)
			fmt.Printf("Auto-accepting transfer: %s from %s\n", payload.FileName, payload.SenderName)

			savePath := filepath.Join(savePathFlag, payload.FileName)

			// Accept the transfer
			receiver := transfer.GetReceiver()
			receiver.AcceptTransfer(payload.TransferID, savePath)

			// Send response back to sender
			response := transfer.TransferResponse{
				TransferID:   payload.TransferID,
				Accepted:     true,
				SavePath:     savePath,
				ReceiverIP:   utils.GetLocalIP(),
				ReceiverPort: 10001,
			}
			go utils.PostJSON(payload.SenderIP, "/transfer/response", response)
		}
	}
}

func runStatus(cmd *cobra.Command, args []string) {
	// Start discovery briefly
	announcement.ListenAndBroadcast()

	fmt.Println("Discovering devices...")
	time.Sleep(2 * time.Second)

	cli.ShowStatus()
}

func runSend(cmd *cobra.Command, args []string) {
	filePath := args[0]

	// Validate file exists
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid file path: %v\n", err)
		os.Exit(1)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if fileInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: cannot send directories\n")
		os.Exit(1)
	}

	// Try to use existing daemon first by querying local API
	var targetIP string
	var targetName string

	// Check if target is an IP address
	if isIPAddress(targetFlag) {
		targetIP = targetFlag
		targetName = targetFlag
	} else {
		// Query local daemon for device list
		devices, err := queryLocalDevices()
		if err != nil {
			// Fall back to starting services
			gin.SetMode(gin.ReleaseMode)
			announcement.ListenAndBroadcast()
			transfer.GetReceiver().StartListener()
			go api.StartServer()

			fmt.Println("Discovering devices...")
			time.Sleep(3 * time.Second)

			devices = announcement.GetOnlineDevices()
		}

		// Find target device
		for _, d := range devices {
			if d.IP == targetFlag || strings.EqualFold(d.Username, targetFlag) {
				targetIP = d.IP
				targetName = d.Username
				break
			}
		}

		if targetIP == "" {
			fmt.Fprintf(os.Stderr, "Error: device '%s' not found\n", targetFlag)
			fmt.Fprintf(os.Stderr, "Online devices:\n")
			for _, d := range devices {
				fmt.Fprintf(os.Stderr, "  %s @ %s\n", d.Username, d.IP)
			}
			os.Exit(1)
		}
	}

	// Initiate transfer
	fmt.Printf("Sending %s to %s (%s)...\n", filepath.Base(absPath), targetName, targetIP)

	// Try to use running daemon to initiate transfer
	transferID, err := initiateViaDaemon(targetIP, absPath)
	if err != nil {
		// Fall back to direct transfer (start own services)
		gin.SetMode(gin.ReleaseMode)
		announcement.ListenAndBroadcast()
		transfer.GetReceiver().StartListener()
		go api.StartServer()
		time.Sleep(1 * time.Second)

		deviceID := utils.GetLocalIP()
		if info, err := utils.GetDeviceInfo(); err == nil {
			deviceID = info.UniqueDeviceID
		}

		sender := transfer.GetSender()
		t, err := sender.InitiateTransfer(
			targetIP,
			absPath,
			deviceID,
			utils.GetUsername(),
			utils.GetLocalIP(),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		sender.SetFilePath(t.ID, absPath)
		transferID = t.ID
	}

	fmt.Println("Transfer request sent. Waiting for acceptance...")

	if !waitFlag {
		fmt.Println("Use --wait to wait for transfer completion")
		return
	}

	// Poll transfer status via daemon API
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastStatus := ""
	for {
		select {
		case <-ticker.C:
			status, err := pollTransferStatus(transferID)
			if err != nil {
				continue
			}

			switch status.Status {
			case "completed":
				fmt.Println("\nTransfer completed!")
				return
			case "failed":
				fmt.Fprintf(os.Stderr, "\nTransfer failed\n")
				os.Exit(1)
			case "rejected":
				fmt.Println("\nTransfer rejected by receiver.")
				os.Exit(1)
			case "in_progress":
				if lastStatus != "in_progress" {
					fmt.Println("Transfer accepted. Sending...")
					lastStatus = "in_progress"
				}
				if status.TotalBytes > 0 {
					percent := float64(status.BytesTransferred) / float64(status.TotalBytes) * 100
					fmt.Printf("\rProgress: %.1f%%", percent)
				}
			}

		case <-timeout:
			fmt.Fprintf(os.Stderr, "\nTimeout waiting for transfer\n")
			os.Exit(1)
		}
	}
}

func initiateViaDaemon(targetIP, filePath string) (string, error) {
	payload := map[string]string{
		"target_ip": targetIP,
		"file_path": filePath,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post("http://localhost:10000/transfer/initiate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	var result struct {
		TransferID string `json:"transfer_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.TransferID, nil
}

type transferStatusResponse struct {
	Status           string `json:"status"`
	BytesTransferred int64  `json:"bytes_transferred"`
	TotalBytes       int64  `json:"total_bytes"`
}

func pollTransferStatus(transferID string) (*transferStatusResponse, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:10000/transfer/status/%s", transferID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var status transferStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func runDevices(cmd *cobra.Command, args []string) {
	announcement.ListenAndBroadcast()
	time.Sleep(2 * time.Second)

	devices := announcement.GetOnlineDevices()
	for _, d := range devices {
		fmt.Printf("%s\t%s\t%s\n", d.Username, d.IP, d.DeviceID[:16])
	}
}

func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func queryLocalDevices() ([]*announcement.Device, error) {
	resp, err := http.Get("http://localhost:10000/devices")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var devices []struct {
		DeviceID string `json:"device_id"`
		Username string `json:"username"`
		IP       string `json:"ip"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, err
	}

	result := make([]*announcement.Device, len(devices))
	for i, d := range devices {
		result[i] = &announcement.Device{
			DeviceID: d.DeviceID,
			Username: d.Username,
			IP:       d.IP,
		}
	}
	return result, nil
}
