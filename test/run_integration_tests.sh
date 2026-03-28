#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Timeout in seconds (default 120 seconds = 2 minutes)
TIMEOUT=${TEST_TIMEOUT:-120}

cd "$PROJECT_DIR"

# Cleanup function
cleanup() {
    echo ""
    echo "=== Cleanup ==="
    docker compose -f docker-compose.test.yml down -v 2>/dev/null || true
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Timeout handler
timeout_handler() {
    echo ""
    echo "=== TEST TIMEOUT (${TIMEOUT}s) ==="
    echo "Tests did not complete within the timeout period."
    exit 1
}

# Start timeout in background
(
    sleep "$TIMEOUT"
    kill -TERM $$ 2>/dev/null
) &
TIMEOUT_PID=$!

# Kill timeout process on exit
trap "kill $TIMEOUT_PID 2>/dev/null; cleanup" EXIT

echo "=== Building Docker images ==="
docker compose -f docker-compose.test.yml build

echo "=== Starting containers ==="
docker compose -f docker-compose.test.yml up -d

echo "=== Waiting for services to start ==="
sleep 5

# Wait for API to be ready (using exposed port on localhost)
for i in {1..30}; do
    if curl -s http://localhost:10010/devices > /dev/null 2>&1; then
        echo "Services are ready!"
        break
    fi
    echo "Waiting for services... ($i/30)"
    sleep 1
done

echo "=== Running tests ==="

# Test 1: Device Discovery
echo ""
echo "--- Test 1: Device Discovery ---"
DEVICES=$(curl -s http://localhost:10010/devices)
echo "Node1 sees devices: $DEVICES"

if echo "$DEVICES" | grep -q "172.28.0.11"; then
    echo "PASS: Node1 discovered Node2"
else
    echo "FAIL: Node1 did not discover Node2"
fi

# Test 2: Check Node2 sees Node1
DEVICES2=$(curl -s http://localhost:10020/devices)
echo "Node2 sees devices: $DEVICES2"

if echo "$DEVICES2" | grep -q "172.28.0.10"; then
    echo "PASS: Node2 discovered Node1"
else
    echo "FAIL: Node2 did not discover Node1"
fi

# Test 3: API Endpoints
echo ""
echo "--- Test 2: API Endpoints ---"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:10010/devices)
if [ "$STATUS" = "200" ]; then
    echo "PASS: /devices returns 200"
else
    echo "FAIL: /devices returned $STATUS"
fi

STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:10010/transfer/status/nonexistent)
if [ "$STATUS" = "404" ]; then
    echo "PASS: /transfer/status returns 404 for unknown transfer"
else
    echo "FAIL: /transfer/status returned $STATUS"
fi

# Test 4: Transfer Request API
echo ""
echo "--- Test 3: Transfer Request API ---"
RESPONSE=$(curl -s -X POST http://localhost:10020/transfer/request \
    -H "Content-Type: application/json" \
    -d '{
        "transfer_id": "test-123",
        "sender_id": "sender-device-id",
        "sender_name": "alice",
        "sender_ip": "172.28.0.10",
        "file_name": "test.txt",
        "file_size": 1024,
        "checksum": "abc123"
    }')

if echo "$RESPONSE" | grep -q "pending"; then
    echo "PASS: Transfer request accepted"
else
    echo "FAIL: Transfer request not accepted: $RESPONSE"
fi

# Test 5: File Transfer (with auto-accept)
echo ""
echo "--- Test 4: File Transfer ---"

# Create test file on node1
docker exec syncd-node1 sh -c 'echo "Hello from syncd integration test!" > /testfiles/hello.txt'
docker exec syncd-node1 sh -c 'cat /testfiles/hello.txt'

# Send file from node1 to node2
echo "Sending file from node1 to node2..."
docker exec syncd-node1 syncd send /testfiles/hello.txt --to 172.28.0.11 --wait &
SEND_PID=$!

# Wait for transfer
sleep 5

# Check if file was received on node2
echo "Checking if file was received on node2..."
if docker exec syncd-node2 test -f /downloads/hello.txt; then
    RECEIVED=$(docker exec syncd-node2 cat /downloads/hello.txt)
    ORIGINAL=$(docker exec syncd-node1 cat /testfiles/hello.txt)
    if [ "$RECEIVED" = "$ORIGINAL" ]; then
        echo "PASS: File transferred successfully and content matches!"
        echo "Content: $RECEIVED"
    else
        echo "FAIL: File content mismatch"
        echo "Original: $ORIGINAL"
        echo "Received: $RECEIVED"
    fi
else
    echo "FAIL: File not found on node2"
    echo "Node2 downloads directory:"
    docker exec syncd-node2 ls -la /downloads/ || true
fi

# Test 6: Large file transfer
echo ""
echo "--- Test 5: Large File Transfer ---"

# Create a 100KB test file in /tmp (not /testfiles to avoid overwriting local files)
docker exec syncd-node1 sh -c 'dd if=/dev/urandom of=/tmp/large.bin bs=1024 count=100 2>/dev/null'
ORIGINAL_SUM=$(docker exec syncd-node1 sha256sum /tmp/large.bin | awk '{print $1}')
echo "Original checksum: $ORIGINAL_SUM"

# Send large file
echo "Sending large file..."
docker exec syncd-node1 syncd send /tmp/large.bin --to 172.28.0.11 --wait &
sleep 10

# Verify
if docker exec syncd-node2 test -f /downloads/large.bin; then
    RECEIVED_SUM=$(docker exec syncd-node2 sha256sum /downloads/large.bin | awk '{print $1}')
    echo "Received checksum: $RECEIVED_SUM"
    if [ "$ORIGINAL_SUM" = "$RECEIVED_SUM" ]; then
        echo "PASS: Large file transferred with matching checksum!"
    else
        echo "FAIL: Checksum mismatch"
    fi
else
    echo "FAIL: Large file not found on node2"
fi

echo ""
echo "=== Tests Complete ==="
