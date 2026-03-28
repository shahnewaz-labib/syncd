package transfer

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"syncd/config"
	"syncd/events"
)

type Receiver struct {
	pendingRequests map[string]*TransferRequest
	acceptedPaths   map[string]string
	listener        net.Listener
	mu              sync.RWMutex
}

var (
	receiverInstance *Receiver
	receiverOnce     sync.Once
	autoAccept       bool
	autoAcceptPath   string
)

func SetAutoAccept(enabled bool, savePath string) {
	autoAccept = enabled
	autoAcceptPath = savePath
}

func IsAutoAccept() bool {
	return autoAccept
}

func GetAutoAcceptPath() string {
	return autoAcceptPath
}

func GetReceiver() *Receiver {
	receiverOnce.Do(func() {
		receiverInstance = &Receiver{
			pendingRequests: make(map[string]*TransferRequest),
			acceptedPaths:   make(map[string]string),
		}
	})
	return receiverInstance
}

func (r *Receiver) AddPendingRequest(request TransferRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingRequests[request.TransferID] = &request
}

func (r *Receiver) GetPendingRequest(transferID string) *TransferRequest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pendingRequests[transferID]
}

func (r *Receiver) AcceptTransfer(transferID, savePath string) {
	r.mu.Lock()
	r.acceptedPaths[transferID] = savePath
	r.mu.Unlock()
}

func (r *Receiver) RejectTransfer(transferID string) {
	r.mu.Lock()
	delete(r.pendingRequests, transferID)
	r.mu.Unlock()
}

func (r *Receiver) StartListener() error {
	addr := fmt.Sprintf(":%d", config.TRANSFER_PORT)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start transfer listener: %w", err)
	}
	r.listener = listener

	go r.acceptConnections()
	return nil
}

func (r *Receiver) acceptConnections() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			continue
		}
		go r.handleConnection(conn)
	}
}

func (r *Receiver) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read transfer ID first
	reader := bufio.NewReader(conn)
	transferID, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	transferID = transferID[:len(transferID)-1] // Remove newline

	r.mu.RLock()
	request := r.pendingRequests[transferID]
	savePath := r.acceptedPaths[transferID]
	r.mu.RUnlock()

	if request == nil || savePath == "" {
		return
	}

	// Create the file
	file, err := os.Create(savePath)
	if err != nil {
		events.PublishFailed(transferID, err)
		return
	}
	defer file.Close()

	// Receive file data
	hash := sha256.New()
	multiWriter := io.MultiWriter(file, hash)

	buf := make([]byte, config.TRANSFER_CHUNK_SIZE)
	var totalReceived int64

	for totalReceived < request.FileSize {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			events.PublishFailed(transferID, err)
			return
		}

		written, err := multiWriter.Write(buf[:n])
		if err != nil {
			events.PublishFailed(transferID, err)
			return
		}

		totalReceived += int64(written)
		events.PublishProgress(transferID, totalReceived, request.FileSize)
	}

	// Verify checksum
	receivedChecksum := fmt.Sprintf("%x", hash.Sum(nil))
	if receivedChecksum != request.Checksum {
		os.Remove(savePath)
		events.PublishFailed(transferID, fmt.Errorf("checksum mismatch"))
		return
	}

	// Cleanup
	r.mu.Lock()
	delete(r.pendingRequests, transferID)
	delete(r.acceptedPaths, transferID)
	r.mu.Unlock()

	events.PublishCompleted(transferID)
}

func (r *Receiver) Stop() {
	if r.listener != nil {
		r.listener.Close()
	}
}
