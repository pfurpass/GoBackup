package core

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/user/backup-tool/utils"
)

// BackupOptions configures a backup run
type BackupOptions struct {
	Source      string // e.g. /dev/sda or /dev/sda1
	Destination string // output .img file path
	ChunkSize   int    // bytes per chunk; 0 → DefaultChunkSize
	Compress    bool
	Encrypt     bool
	Password    string // required when Encrypt == true
}

// Progress is sent on the channel during backup/restore
type Progress struct {
	BytesDone  uint64
	BytesTotal uint64
	SpeedBPS   float64 // bytes per second
	ETA        time.Duration
	Message    string
	Error      error
	Done       bool
}

// BackupEngine executes a backup
type BackupEngine struct {
	opts   BackupOptions
	logger *utils.Logger
}

// NewBackupEngine creates a BackupEngine
func NewBackupEngine(opts BackupOptions, logger *utils.Logger) *BackupEngine {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = DefaultChunkSize
	}
	return &BackupEngine{opts: opts, logger: logger}
}

// Run performs the backup, reporting progress on ch.
// The caller must drain ch; close is called when done.
func (e *BackupEngine) Run(ch chan<- Progress) {
	defer close(ch)

	send := func(p Progress) { ch <- p }
	fail := func(err error) {
		e.logger.Errorf("Backup fehler: %v", err)
		send(Progress{Error: err, Done: true})
	}
	log := func(msg string) {
		e.logger.Infof(msg)
		send(Progress{Message: msg})
	}

	// ── Open source ──────────────────────────────────────────────────────────
	log(fmt.Sprintf("Öffne Quelle: %s", e.opts.Source))
	src, err := os.OpenFile(e.opts.Source, os.O_RDONLY, 0)
	if err != nil {
		fail(fmt.Errorf("Quelle öffnen: %w", err))
		return
	}
	defer src.Close()

	// Get device/file size
	total, err := getDeviceSize(src)
	if err != nil {
		fail(fmt.Errorf("Gerätegröße ermitteln: %w", err))
		return
	}
	log(fmt.Sprintf("Quellgröße: %s", FormatSize(total)))

	// ── Prepare encryption ───────────────────────────────────────────────────
	var salt [32]byte
	var key []byte
	if e.opts.Encrypt {
		if _, err := rand.Read(salt[:]); err != nil {
			fail(fmt.Errorf("Salt erzeugen: %w", err))
			return
		}
		key = utils.DeriveKey(e.opts.Password, salt[:])
		log("AES-256-GCM Verschlüsselung aktiv")
	}

	// ── Build backup header ───────────────────────────────────────────────────
	var hdr BackupHeader
	copy(hdr.Magic[:], BackupMagic)
	hdr.Version = FileVersion
	if e.opts.Compress {
		hdr.Flags |= FlagCompressed
	}
	if e.opts.Encrypt {
		hdr.Flags |= FlagEncrypted
	}
	copy(hdr.DevicePath[:], e.opts.Source)
	hdr.DeviceSize = total
	hdr.ChunkSize = uint32(e.opts.ChunkSize)
	hdr.Salt = salt

	// ── Open destination ─────────────────────────────────────────────────────
	log(fmt.Sprintf("Erstelle Backup-Datei: %s", e.opts.Destination))
	dst, err := os.OpenFile(e.opts.Destination,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fail(fmt.Errorf("Zieldatei erstellen: %w", err))
		return
	}
	defer dst.Close()

	// Write placeholder header (we'll rewrite DataHash at the end)
	if err := binary.Write(dst, binary.LittleEndian, &hdr); err != nil {
		fail(fmt.Errorf("Header schreiben: %w", err))
		return
	}
	// Pad header to HeaderSize
	written := binary.Size(hdr)
	if pad := HeaderSize - written; pad > 0 {
		if _, err := dst.Write(make([]byte, pad)); err != nil {
			fail(fmt.Errorf("Header-Padding: %w", err))
			return
		}
	}

	// ── Chunk loop ────────────────────────────────────────────────────────────
	hasher := utils.NewHasher()
	buf := make([]byte, e.opts.ChunkSize)
	var done uint64
	startTime := time.Now()
	lastReport := startTime

	for {
		n, readErr := io.ReadFull(src, buf)
		chunk := buf[:n]
		if n == 0 {
			break
		}

		// Running checksum of original data
		hasher.Write(chunk)

		chunkHash := utils.SHA256(chunk)

		// Compress
		stored := chunk
		if e.opts.Compress {
			compressed, cerr := utils.CompressChunk(chunk)
			if cerr != nil {
				fail(fmt.Errorf("Kompression Chunk %d: %w", done, cerr))
				return
			}
			stored = compressed
		}

		// Encrypt
		var nonce [12]byte
		if e.opts.Encrypt {
			if _, err := rand.Read(nonce[:]); err != nil {
				fail(fmt.Errorf("Nonce erzeugen: %w", err))
				return
			}
			enc, eerr := utils.EncryptChunk(stored, key, nonce[:])
			if eerr != nil {
				fail(fmt.Errorf("Verschlüsselung Chunk: %w", eerr))
				return
			}
			stored = enc
		}

		// Write chunk header
		chHdr := ChunkHeader{
			OrigSize:  uint32(n),
			StoreSize: uint32(len(stored)),
			Nonce:     nonce,
		}
		copy(chHdr.ChunkHash[:], chunkHash)
		if err := binary.Write(dst, binary.LittleEndian, &chHdr); err != nil {
			fail(fmt.Errorf("Chunk-Header schreiben: %w", err))
			return
		}

		// Write chunk data
		if _, err := dst.Write(stored); err != nil {
			fail(fmt.Errorf("Chunk-Daten schreiben: %w", err))
			return
		}

		done += uint64(n)
		now := time.Now()

		if now.Sub(lastReport) >= 500*time.Millisecond || readErr != nil {
			elapsed := now.Sub(startTime).Seconds()
			var speed float64
			if elapsed > 0 {
				speed = float64(done) / elapsed
			}
			var eta time.Duration
			if speed > 0 && total > done {
				eta = time.Duration(float64(total-done)/speed) * time.Second
			}
			send(Progress{
				BytesDone:  done,
				BytesTotal: total,
				SpeedBPS:   speed,
				ETA:        eta,
				Message:    fmt.Sprintf("%s / %s (%.1f MB/s)", FormatSize(done), FormatSize(total), speed/1024/1024),
			})
			lastReport = now
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			e.logger.Warnf("Lesefehler bei Offset %d: %v (überspringe Chunk)", done, readErr)
		}
	}

	// ── Finalize header with DataHash ─────────────────────────────────────────
	dataHash := hasher.Sum(nil)
	copy(hdr.DataHash[:], dataHash)

	if _, err := dst.Seek(0, io.SeekStart); err != nil {
		fail(fmt.Errorf("Seek zum Header: %w", err))
		return
	}
	if err := binary.Write(dst, binary.LittleEndian, &hdr); err != nil {
		fail(fmt.Errorf("Header finalisieren: %w", err))
		return
	}

	log(fmt.Sprintf("✅ Backup abgeschlossen: %s", FormatSize(done)))
	send(Progress{BytesDone: done, BytesTotal: total, Done: true,
		Message: fmt.Sprintf("Backup erfolgreich! %s gesichert.", FormatSize(done))})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// getDeviceSize returns the byte size of a file or block device
func getDeviceSize(f *os.File) (uint64, error) {
	// Try stat first (works for regular files)
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if fi.Mode().IsRegular() {
		return uint64(fi.Size()), nil
	}

	// Block device: use BLKGETSIZE64 ioctl
	const BLKGETSIZE64 = 0x80081272
	var size uint64
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		f.Fd(), BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, fmt.Errorf("BLKGETSIZE64 ioctl: %w", errno)
	}
	return size, nil
}

// nullSafe reads a null-terminated string from a byte slice
func nullSafe(b []byte) string {
	idx := bytes.IndexByte(b, 0)
	if idx < 0 {
		return string(b)
	}
	return string(b[:idx])
}
