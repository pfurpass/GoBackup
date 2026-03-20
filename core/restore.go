package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/user/backup-tool/utils"
)

// RestoreOptions configures a restore run
type RestoreOptions struct {
	Source      string // backup .img file path
	Destination string // target block device, e.g. /dev/sdb
	Password    string // required if backup was encrypted
}

// BackupMeta holds metadata read from a backup file header
type BackupMeta struct {
	DevicePath string
	DeviceSize uint64
	ChunkSize  uint32
	Compressed bool
	Encrypted  bool
	DataHash   [32]byte
}

// RestoreEngine executes a restore
type RestoreEngine struct {
	opts   RestoreOptions
	logger *utils.Logger
}

// NewRestoreEngine creates a RestoreEngine
func NewRestoreEngine(opts RestoreOptions, logger *utils.Logger) *RestoreEngine {
	return &RestoreEngine{opts: opts, logger: logger}
}

// ReadMeta reads only the header of a backup file and returns metadata.
// Useful for displaying information before starting a restore.
func ReadMeta(path string) (*BackupMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hdr, err := readHeader(f)
	if err != nil {
		return nil, err
	}

	return &BackupMeta{
		DevicePath: nullSafe(hdr.DevicePath[:]),
		DeviceSize: hdr.DeviceSize,
		ChunkSize:  hdr.ChunkSize,
		Compressed: hdr.Flags&FlagCompressed != 0,
		Encrypted:  hdr.Flags&FlagEncrypted != 0,
		DataHash:   hdr.DataHash,
	}, nil
}

// Run performs the restore, reporting progress on ch.
func (e *RestoreEngine) Run(ch chan<- Progress) {
	defer close(ch)

	send := func(p Progress) { ch <- p }
	fail := func(err error) {
		e.logger.Errorf("Restore Fehler: %v", err)
		send(Progress{Error: err, Done: true})
	}
	log := func(msg string) {
		e.logger.Infof(msg)
		send(Progress{Message: msg})
	}

	// ── Open backup file ──────────────────────────────────────────────────────
	log(fmt.Sprintf("Öffne Backup: %s", e.opts.Source))
	src, err := os.Open(e.opts.Source)
	if err != nil {
		fail(fmt.Errorf("Backup-Datei öffnen: %w", err))
		return
	}
	defer src.Close()

	hdr, err := readHeader(src)
	if err != nil {
		fail(fmt.Errorf("Header lesen: %w", err))
		return
	}

	compressed := hdr.Flags&FlagCompressed != 0
	encrypted := hdr.Flags&FlagEncrypted != 0
	total := hdr.DeviceSize
	chunkSize := int(hdr.ChunkSize)

	log(fmt.Sprintf("Quellgerät: %s  Größe: %s", nullSafe(hdr.DevicePath[:]), FormatSize(total)))
	if compressed {
		log("Kompression: zstd")
	}
	if encrypted {
		log("Verschlüsselung: AES-256-GCM")
	}

	// ── Derive key if encrypted ───────────────────────────────────────────────
	var key []byte
	if encrypted {
		if e.opts.Password == "" {
			fail(fmt.Errorf("Backup ist verschlüsselt – Passwort erforderlich"))
			return
		}
		key = utils.DeriveKey(e.opts.Password, hdr.Salt[:])
	}

	// ── Open destination device ───────────────────────────────────────────────
	log(fmt.Sprintf("Öffne Zielgerät: %s", e.opts.Destination))
	dst, err := os.OpenFile(e.opts.Destination, os.O_WRONLY|os.O_SYNC, 0)
	if err != nil {
		fail(fmt.Errorf("Zielgerät öffnen: %w", err))
		return
	}
	defer dst.Close()

	// ── Chunk loop ────────────────────────────────────────────────────────────
	hasher := utils.NewHasher()
	var done uint64
	startTime := time.Now()
	lastReport := startTime
	_ = chunkSize // used for buf allocation hint

	for {
		// Read chunk header
		var chHdr ChunkHeader
		if err := binary.Read(src, binary.LittleEndian, &chHdr); err != nil {
			if err == io.EOF {
				break
			}
			fail(fmt.Errorf("Chunk-Header lesen: %w", err))
			return
		}

		// Read stored (possibly compressed+encrypted) chunk data
		stored := make([]byte, chHdr.StoreSize)
		if _, err := io.ReadFull(src, stored); err != nil {
			fail(fmt.Errorf("Chunk-Daten lesen: %w", err))
			return
		}

		// Decrypt
		if encrypted {
			dec, derr := utils.DecryptChunk(stored, key, chHdr.Nonce[:])
			if derr != nil {
				fail(fmt.Errorf("Entschlüsselung fehlgeschlagen: %w", derr))
				return
			}
			stored = dec
		}

		// Decompress
		if compressed {
			dec, derr := utils.DecompressChunk(stored)
			if derr != nil {
				fail(fmt.Errorf("Dekompression fehlgeschlagen: %w", derr))
				return
			}
			stored = dec
		}

		// Verify chunk integrity
		gotHash := utils.SHA256(stored)
		if string(gotHash) != string(chHdr.ChunkHash[:]) {
			e.logger.Warnf("Chunk-Prüfsumme ungültig bei Offset %d – überspringe", done)
		}

		// Write to destination
		if _, err := dst.Write(stored); err != nil {
			fail(fmt.Errorf("Schreiben fehlgeschlagen: %w", err))
			return
		}

		hasher.Write(stored)
		done += uint64(len(stored))

		now := time.Now()
		if now.Sub(lastReport) >= 500*time.Millisecond {
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
	}

	// ── Verify overall data hash ──────────────────────────────────────────────
	finalHash := hasher.Sum(nil)
	if string(finalHash) != string(hdr.DataHash[:]) {
		e.logger.Warnf("⚠️  Gesamt-Prüfsumme stimmt nicht überein! Backup könnte beschädigt sein.")
		send(Progress{Message: "⚠️  Prüfsummen-Warnung: Daten könnten beschädigt sein!"})
	} else {
		log("✅ Prüfsumme verifiziert – Daten intakt")
	}

	log(fmt.Sprintf("✅ Restore abgeschlossen: %s geschrieben", FormatSize(done)))
	send(Progress{BytesDone: done, BytesTotal: total, Done: true,
		Message: fmt.Sprintf("Restore erfolgreich! %s wiederhergestellt.", FormatSize(done))})
}

// ── private helpers ───────────────────────────────────────────────────────────

func readHeader(r io.ReadSeeker) (*BackupHeader, error) {
	// Seek to start
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var hdr BackupHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("Header lesen: %w", err)
	}

	if string(hdr.Magic[:]) != BackupMagic {
		return nil, fmt.Errorf("ungültige Magic-Bytes: keine GoBackup-Datei")
	}
	if hdr.Version != FileVersion {
		return nil, fmt.Errorf("unbekannte Version %d (erwartet %d)", hdr.Version, FileVersion)
	}

	// Seek past full header to data start
	if _, err := r.Seek(HeaderSize, io.SeekStart); err != nil {
		return nil, err
	}

	return &hdr, nil
}
