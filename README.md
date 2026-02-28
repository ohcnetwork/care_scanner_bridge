# Care Scanner Bridge

A lightweight, cross-platform desktop application that bridges serial barcode scanners to web applications via WebSocket.

## Features

- 🖥️ **Cross-platform**: Works on macOS, Windows, and Linux
- 🔌 **Plug & Play**: Auto-detects serial barcode scanners
- 🌐 **WebSocket Server**: Exposes scanner data to web apps on `localhost:7001`
- 📡 **System Tray**: Runs silently in the background
- ⚡ **Lightweight**: Single binary, no dependencies (~10MB)
- 🔒 **Secure**: Only accepts connections from localhost

## Installation

### macOS

1. Download `care-scanner-bridge-macos.dmg` from [Releases](https://github.com/ohcnetwork/care_scanner_bridge/releases)
2. Open the DMG and drag "Care Scanner Bridge" to Applications
3. **First launch (Important - the app is not notarized):**
   - **Option A:** Right-click on the app in Applications → Select "Open" → Click "Open" in the dialog
   - **Option B:** If you see "not responding" or it won't open, run this command in Terminal:

     ```bash
     xattr -cr /Applications/Care\ Scanner\ Bridge.app && open /Applications/Care\ Scanner\ Bridge.app
     ```

   - **Option C:** Go to System Settings → Privacy & Security → Scroll down and click "Open Anyway"

### Windows

1. Download `care-scanner-bridge-setup.exe` from [Releases](https://github.com/ohcnetwork/care_scanner_bridge/releases)
2. Run the installer (click "More info" → "Run anyway" if Windows SmartScreen appears)
3. Launch from Start Menu or Desktop shortcut

### Linux

1. Download `care-scanner-bridge-linux.AppImage` from [Releases](https://github.com/ohcnetwork/care_scanner_bridge/releases)
2. Make it executable: `chmod +x care-scanner-bridge-linux.AppImage`
3. Run: `./care-scanner-bridge-linux.AppImage`

## Usage

1. **Connect your barcode scanner** via USB
2. **Launch Care Scanner Bridge** - it will appear in your system tray
3. **Select the scanner port** from the tray menu
4. **Your web application** can now connect to `ws://localhost:7001/ws`

## WebSocket API

### Connect to the server

```javascript
const ws = new WebSocket('ws://localhost:7001/ws');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  switch (message.type) {
    case 'scan':
      console.log('Barcode scanned:', message.payload.barcode);
      break;
    case 'status':
      console.log('Connected:', message.payload.connected);
      break;
  }
};
```

### Message Types

#### Incoming (from server)

| Type | Payload | Description |
|------|---------|-------------|
| `scan` | `{ barcode, port, timestamp }` | A barcode was scanned |
| `status` | `{ connected, currentPort }` | Connection status update |
| `ports` | `[{ path, description, isConnected }]` | List of available ports |
| `error` | `{ message }` | Error occurred |
| `pong` | - | Response to ping |

#### Outgoing (to server)

| Type | Payload | Description |
|------|---------|-------------|
| `connect` | `{ port, baudRate? }` | Connect to a port |
| `disconnect` | - | Disconnect from current port |
| `list_ports` | - | Request list of available ports |
| `status` | - | Request current status |
| `ping` | - | Keep-alive ping |

### Health Check

```bash
curl http://localhost:7001/health
```

Returns:
```json
{
  "status": "ok",
  "connected": true,
  "port": "/dev/cu.usbserial-1234"
}
```

## Configuration

Configuration is stored in:
- **macOS/Linux**: `~/.care_scanner_bridge/config.json`
- **Windows**: `%USERPROFILE%\.care_scanner_bridge\config.json`

```json
{
  "port": 7001,
  "baudRate": 9600,
  "autoConnect": true,
  "lastDevice": "/dev/cu.usbserial-1234",
  "startMinimized": false
}
```

## Building from Source

### Prerequisites

- Go 1.21 or later
- For Linux: `libgtk-3-dev`, `libayatana-appindicator3-dev`

### Build

```bash
# Clone the repository
git clone https://github.com/ohcnetwork/care_scanner_bridge.git
cd care_scanner_bridge

# Download dependencies
go mod download

# Build
go build -o care-scanner-bridge .

# Run
./care-scanner-bridge
```

### Cross-compile

```bash
# For macOS (from Linux/Windows)
GOOS=darwin GOARCH=amd64 go build -o care-scanner-bridge-macos .

# For Windows (from Linux/macOS)
GOOS=windows GOARCH=amd64 go build -o care-scanner-bridge.exe .

# For Linux (from macOS/Windows)
GOOS=linux GOARCH=amd64 go build -o care-scanner-bridge-linux .
```

## Troubleshooting

### Scanner not detected

1. Ensure the scanner is in "USB Serial" mode (not HID/Keyboard mode)
2. Check if the port appears in the system:
   - macOS: `ls /dev/cu.*`
   - Linux: `ls /dev/ttyUSB* /dev/ttyACM*`
   - Windows: Device Manager → Ports (COM & LPT)
3. Try unplugging and reconnecting the scanner

### Permission denied (Linux)

Add your user to the `dialout` group:
```bash
sudo usermod -a -G dialout $USER
# Log out and back in
```

### macOS security warning

Right-click the app → Open, then click "Open" in the dialog.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.
