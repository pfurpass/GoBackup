# 🔷 GoBackup

**Block-Level Disk Backup & Restore for Linux**  
A professional open-source alternative to Acronis True Image – built entirely in Go with a native GUI.

---

## Screenshots

```
┌─────────────────────────────────────────────────────────┐
│  🔷 GoBackup  |  Block-Level Disk Backup for Linux      │
├──────────┬──────────────────────────────────────────────┤
│          │  Detected Drives                             │
│  Disks   │  ┌────────────┬──────────────┬──────┬──────┐ │
│          │  │ Device     │ Model        │ Type │ Size │ │
│  Backup  │  ├────────────┼──────────────┼──────┼──────┤ │
│          │  │ /dev/sda   │ Samsung 870  │ SSD  │ 1 TB │ │
│  Restore │  │  > sda1    │ —            │ Part │ 512G │ │
│          │  │  > sda2    │ —            │ Part │ 488G │ │
│  Log     │  └────────────┴──────────────┴──────┴──────┘ │
└──────────┴──────────────────────────────────────────────┘
```

---

## Features

| Feature | Details |
|---|---|
| **Block-Level Backup** | Direct access to `/dev/sda`, `/dev/nvme0n1`, etc. |
| **Compression** | zstd – up to 60% smaller images |
| **Encryption** | AES-256-GCM + PBKDF2-SHA256 (200,000 iterations) |
| **Checksums** | SHA-256 per chunk + full image verification |
| **Streaming** | Chunk-based (1 MB), no RAM overflow |
| **Progress** | Real-time: MB/s, ETA, live log |
| **GUI** | Fyne v2 – dark theme, native |
| **Safety Lock** | Automatically protects the active system drive |
| **Error Tolerance** | Bad blocks are skipped with a warning |

---

## Requirements

### Debian / Ubuntu
```bash
sudo apt install -y golang-go gcc libgl1-mesa-dev xorg-dev libx11-dev
```

### Minimum Requirements
- Go 1.21+
- Linux (Debian 12+ recommended)
- Root privileges for disk access
- 512 MB RAM
- X11 or Wayland

---

## Installation

```bash
# Clone the repository
git clone https://github.com/user/backup-tool
cd backup-tool

# Download dependencies
go mod tidy

# Build
bash install.sh
 
# Run (root required)
sudo ./backup-tool

---

## Usage

### Creating a Backup

1. Go to the **Disks** tab → browse detected drives
2. Go to the **Backup** tab → select source device and destination file
3. Set options: compression (zstd) and/or encryption (AES-256)
4. Click **Start Backup**
5. Monitor real-time progress in the **Log** tab

### Restoring a Backup

1. Go to the **Restore** tab → open a backup image
2. Select the target device
3. ⚠️ Confirm the safety warning
4. Click **Start Restore**

---

## Backup File Format

``
┌─────────────────────────────────────┐
│  BACKUP HEADER (512 bytes)          │
│  Magic:      "GOBACKUP"             │
│  Version:    uint32                 │
│  Flags:      Compression | Enc.     │
│  DevicePath: source device          │
│  DeviceSize: uint64                 │
│  Salt:       [32]byte (PBKDF2)      │
│  DataHash:   [32]byte (SHA-256)     │
├─────────────────────────────────────┤
│  CHUNK #1 HEADER (52 bytes)         │
│  OrigSize / StoreSize               │
│  Nonce:  [12]byte (AES-GCM)         │
│  Hash:   [32]byte (SHA-256)         │
├─────────────────────────────────────┤
│  CHUNK #1 DATA                      │
│  (compressed + encrypted)           │
├─────────────────────────────────────┤
│  CHUNK #2 …                         │
└─────────────────────────────────────┘
```

---

## Project Structure

```
backup-tool/
├── main.go               # Entry point
├── install.sh            # Build & install script
│
├── core/                 # Backup engine
│   ├── format.go         # Binary format definitions
│   ├── disk.go           # Disk detection (lsblk)
│   ├── backup.go         # Backup engine
│   ├── restore.go        # Restore engine
│   └── safety.go         # Safety locks
│
├── utils/                # Shared libraries
│   ├── compression.go    # zstd
│   ├── crypto.go         # AES-256-GCM
│   ├── checksum.go       # SHA-256
│   └── logger.go         # Logging
│
└── ui/                   # Fyne GUI
    ├── app.go            # Main window & navigation
    ├── disk_screen.go    # Disk overview
    ├── backup_screen.go  # Backup configuration
    ├── restore_screen.go # Restore + confirmation
    └── progress_screen.go# Progress & live log
```

---

## Live USB

GoBackup can boot directly from a USB stick.
ISO -> Releases

Boot sequence:
```
Power ON → GRUB → Debian (no login) → GoBackup starts automatically
```

---

## Security

- **AES-256-GCM**: Authenticated encryption (confidentiality + integrity)
- **PBKDF2-SHA256**: 200,000 iterations for key derivation
- **Random Salt**: 32 bytes per backup (crypto/rand)
- **Per-Chunk Nonce**: Unique 12-byte GCM nonce per chunk
- **Double Confirmation**: Restore requires explicit user confirmation
- **System Protection**: Active system drive cannot be overwritten

---

## Architecture

```
┌─────────────────────────────────────────┐
│  ui/  (Fyne GUI)                        │
│  DiskScreen │ BackupScreen │ RestoreScr.│
└──────────────────┬──────────────────────┘
                   │ chan Progress
┌──────────────────▼──────────────────────┐
│  core/  (Engine – no UI imports)        │
│  BackupEngine    │    RestoreEngine     │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│  utils/  (crypto, compression, hash)    │
└─────────────────────────────────────────┘
```

**Key principle:** `ui/` never performs disk I/O directly.  
Communication is handled exclusively via `chan core.Progress` and options structs.

---

## License

Apache

---

## Built With

- [Go](https://golang.org) – Programming language
- [Fyne](https://fyne.io) – GUI framework
- [klauspost/compress](https://github.com/klauspost/compress) – zstd compression
- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) – AES-256-GCM encryption
