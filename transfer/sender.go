package transfer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"syncd/config"
	"syncd/events"
)

type Sender struct {
	pendingTransfers map[string]*Transfer
	mu               sync.RWMutex
}

var (
	senderInstance *Sender
	senderOnce     sync.Once
)

func GetSender() *Sender {
	senderOnce.Do(func() {
		senderInstance = &Sender{
			pendingTransfers: make(map[string]*Transfer),
		}
	})
	return senderInstance
}

func (s *Sender) InitiateTransfer(targetIP string, filePath string, senderID, senderName, senderIP string) (*Transfer, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	if fileInfo.IsDir() {
		return nil, fmt.Errorf("cannot transfer directories")
	}

	checksum, err := CalculateChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	transferID := GenerateTransferID()
	request := TransferRequest{
		TransferID: transferID,
		SenderID:   senderID,
		SenderName: senderName,
		SenderIP:   senderIP,
		FileName:   filepath.Base(filePath),
		FileSize:   fileInfo.Size(),
		Checksum:   checksum,
	}

	transfer := &Transfer{
		ID:        transferID,
		Request:   request,
		Status:    StatusPending,
		StartTime: time.Now(),
	}

	s.mu.Lock()
	s.pendingTransfers[transferID] = transfer
	s.mu.Unlock()

	// Send request to receiver
	url := fmt.Sprintf("http://%s:%s/transfer/request", targetIP, config.API_PORT)
	body, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		s.updateTransferStatus(transferID, StatusFailed, err)
		return nil, fmt.Errorf("failed to send transfer request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("transfer request rejected with status %d", resp.StatusCode)
		s.updateTransferStatus(transferID, StatusFailed, err)
		return nil, err
	}

	return transfer, nil
}

func (s *Sender) HandleResponse(response TransferResponse) {
	s.mu.Lock()
	transfer, exists := s.pendingTransfers[response.TransferID]
	if !exists {
		s.mu.Unlock()
		return
	}
	transfer.Response = &response
	s.mu.Unlock()

	if !response.Accepted {
		s.updateTransferStatus(response.TransferID, StatusRejected, nil)
		events.PublishFailed(response.TransferID, fmt.Errorf("transfer rejected"))
		return
	}

	// Start file transfer in background
	go s.streamFile(transfer, response)
}

func (s *Sender) streamFile(transfer *Transfer, response TransferResponse) {
	s.updateTransferStatus(transfer.ID, StatusInProgress, nil)

	addr := fmt.Sprintf("%s:%d", response.ReceiverIP, response.ReceiverPort)
	conn, err := net.DialTimeout("tcp", addr, config.TRANSFER_TIMEOUT)
	if err != nil {
		s.updateTransferStatus(transfer.ID, StatusFailed, err)
		events.PublishFailed(transfer.ID, err)
		return
	}
	defer conn.Close()

	// Send transfer ID first
	_, err = conn.Write([]byte(transfer.ID + "\n"))
	if err != nil {
		s.updateTransferStatus(transfer.ID, StatusFailed, err)
		events.PublishFailed(transfer.ID, err)
		return
	}

	// Find the original file path (stored when initiating)
	filePath := s.getFilePath(transfer.ID)
	if filePath == "" {
		err := fmt.Errorf("file path not found for transfer")
		s.updateTransferStatus(transfer.ID, StatusFailed, err)
		events.PublishFailed(transfer.ID, err)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		s.updateTransferStatus(transfer.ID, StatusFailed, err)
		events.PublishFailed(transfer.ID, err)
		return
	}
	defer file.Close()

	buf := make([]byte, config.TRANSFER_CHUNK_SIZE)
	var totalSent int64

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			s.updateTransferStatus(transfer.ID, StatusFailed, err)
			events.PublishFailed(transfer.ID, err)
			return
		}

		written, err := conn.Write(buf[:n])
		if err != nil {
			s.updateTransferStatus(transfer.ID, StatusFailed, err)
			events.PublishFailed(transfer.ID, err)
			return
		}

		totalSent += int64(written)
		s.updateProgress(transfer.ID, totalSent)
		events.PublishProgress(transfer.ID, totalSent, transfer.Request.FileSize)
	}

	s.updateTransferStatus(transfer.ID, StatusCompleted, nil)
	events.PublishCompleted(transfer.ID)
}

func (s *Sender) updateTransferStatus(transferID string, status TransferStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transfer, exists := s.pendingTransfers[transferID]; exists {
		transfer.Status = status
		transfer.Error = err
		if status == StatusCompleted || status == StatusFailed || status == StatusRejected {
			transfer.EndTime = time.Now()
		}
	}
}

func (s *Sender) updateProgress(transferID string, bytesTransferred int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transfer, exists := s.pendingTransfers[transferID]; exists {
		transfer.BytesTransferred = bytesTransferred
	}
}

func (s *Sender) GetTransfer(transferID string) *Transfer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pendingTransfers[transferID]
}

// File path storage for transfers
var (
	transferFilePaths = make(map[string]string)
	filePathMu        sync.RWMutex
)

func (s *Sender) SetFilePath(transferID, filePath string) {
	filePathMu.Lock()
	defer filePathMu.Unlock()
	transferFilePaths[transferID] = filePath
}

func (s *Sender) getFilePath(transferID string) string {
	filePathMu.RLock()
	defer filePathMu.RUnlock()
	return transferFilePaths[transferID]
}
