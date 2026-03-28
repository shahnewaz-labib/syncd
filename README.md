# syncd

peer-to-peer file transfer for local networks. discover devices automatically and send files with a simple accept/reject flow.

## features

- automatic device discovery via udp broadcast
- interactive tui dashboard
- cli for scripting
- checksum verification for file integrity
- headless daemon mode with auto-accept

## architecture

```
+------------------+                    +------------------+
|     device a     |                    |     device b     |
+------------------+                    +------------------+
|                  |                    |                  |
|  +------------+  |   udp broadcast    |  +------------+  |
|  | discovery  |<-+------------------->+->| discovery  |  |
|  +------------+  |   (port 9999)      |  +------------+  |
|                  |                    |                  |
|  +------------+  |   http signaling   |  +------------+  |
|  |  api       |<-+------------------->+->|  api       |  |
|  +------------+  |   (port 10000)     |  +------------+  |
|                  |                    |                  |
|  +------------+  |   tcp streaming    |  +------------+  |
|  | transfer   |--+------------------->+->| transfer   |  |
|  +------------+  |   (port 10001)     |  +------------+  |
|                  |                    |                  |
|  +------------+  |                    |  +------------+  |
|  | tui / cli  |  |                    |  | tui / cli  |  |
|  +------------+  |                    |  +------------+  |
+------------------+                    +------------------+
```

## device discovery

devices announce themselves via udp broadcast every 5 seconds:

```
+----------+          +----------+          +----------+
| device a |          | network  |          | device b |
+----------+          +----------+          +----------+
     |                     |                     |
     |---> broadcast ----->|                     |
     |    {device_id,      |---> deliver ------->|
     |     username,       |                     |
     |     ip}             |                     |
     |                     |                     |
     |                     |<--- broadcast <-----|
     |<--- deliver <-------|    {device_id,      |
     |                     |     username,       |
     |                     |     ip}             |
```

each device maintains a list of online peers, expiring entries after 15 seconds of silence.

## file transfer flow

```
    sender                                    receiver
       |                                          |
       |  1. POST /transfer/request               |
       |  {transfer_id, filename, size, checksum} |
       |----------------------------------------->|
       |                                          |
       |                          [user prompted] |
       |                          [accept/reject] |
       |                                          |
       |  2. POST /transfer/response              |
       |  {accepted: true, save_path, port}       |
       |<-----------------------------------------|
       |                                          |
       |  3. tcp connect to receiver:10001        |
       |=========================================>|
       |                                          |
       |  4. stream file chunks (64kb)            |
       |=========================================>|
       |                                          |
       |                    [verify sha256]       |
       |                    [save to disk]        |
```

### transfer details

**1. initiation (sender)**

before sending, the sender:
- calculates sha256 checksum of the file
- generates a unique 16-char transfer id
- sends metadata via http to receiver's `/transfer/request`

```
transfer request payload:
{
  "transfer_id": "a1b2c3d4e5f67890",
  "sender_id":   "device-unique-id",
  "sender_name": "alice",
  "sender_ip":   "192.168.1.10",
  "file_name":   "document.pdf",
  "file_size":   1048576,
  "checksum":    "e3b0c44298fc1c14..."
}
```

**2. acceptance (receiver)**

the receiver stores the pending request and prompts the user (or auto-accepts in daemon mode). on accept:
- chooses save path
- responds via http to sender's `/transfer/response`

```
transfer response payload:
{
  "transfer_id":   "a1b2c3d4e5f67890",
  "accepted":      true,
  "save_path":     "/downloads/document.pdf",
  "receiver_ip":   "192.168.1.20",
  "receiver_port": 10001
}
```

**3. tcp streaming**

```
sender                                         receiver
   |                                               |
   |  [open tcp connection to receiver:10001]      |
   |---------------------------------------------->|
   |                                               |
   |  [send transfer_id + newline]                 |
   |  "a1b2c3d4e5f67890\n"                         |
   |---------------------------------------------->|
   |                                               |
   |  [read file in 64kb chunks]                   |
   |                                               |
   |  chunk 1 (64kb)                               |
   |---------------------------------------------->|
   |  chunk 2 (64kb)                               |  [write to file]
   |---------------------------------------------->|  [update sha256 hash]
   |  ...                                          |
   |  chunk n (remaining bytes)                    |
   |---------------------------------------------->|
   |                                               |
   |  [close connection]                           |  [verify checksum]
   |---------------------------------------------->|  [complete or delete]
```

**4. verification (receiver)**

the receiver computes sha256 while writing:

```go
hash := sha256.New()
multiWriter := io.MultiWriter(file, hash)
// all writes go to both file and hash
```

after receiving all bytes, it compares checksums:
- match: transfer complete, keep file
- mismatch: delete file, report failure

**status tracking**

transfers go through these states:

```
pending --> in_progress --> completed
                |
                +--> failed
                |
                +--> rejected
```

events are published to an internal channel so the tui/cli can show real-time progress.

## usage

### tui dashboard

```bash
syncd
```

launches the interactive dashboard with three panes:
- devices: online peers on the network
- transfers: active/completed transfers
- logs: system events

keyboard shortcuts:
- `tab` - switch panes
- `enter` - select device / accept transfer
- `n` - reject transfer
- `s` - send file (when device selected)
- `q` - quit

### cli commands

```bash
# send a file to a device by ip
syncd send myfile.txt --to 192.168.1.10

# send a file to a device by username
syncd send myfile.txt --to john

# wait for transfer to complete
syncd send myfile.txt --to john --wait

# list online devices
syncd devices

# show status
syncd status
```

### daemon mode

run headless for receiving files without the tui:

```bash
# basic daemon
syncd daemon

# auto-accept all incoming transfers
syncd daemon --auto-accept --save-path /downloads
```

## ports

| port  | protocol | purpose              |
|-------|----------|----------------------|
| 9999  | udp      | device discovery     |
| 10000 | http     | api / signaling      |
| 10001 | tcp      | file transfer        |

## project structure

```
syncd/
├── announcement/     # udp broadcast discovery
├── api/              # http endpoints (gin)
├── cli/              # tui dashboard (bubbletea)
│   └── ui/           # ui components
├── cmd/              # cobra commands
├── config/           # constants
├── events/           # event bus for cli<->api
├── transfer/         # file transfer protocol
└── utils/            # helpers (device id, networking)
```

## building

```bash
go build -o syncd .
```

## testing

run integration tests with docker:

```bash
./test/run_integration_tests.sh
```

this spins up two containers on a virtual network and tests:
- device discovery between nodes
- api endpoints
- file transfer with checksum verification
