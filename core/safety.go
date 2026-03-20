package core

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SafetyInfo holds devices that must never be used as backup source
type SafetyInfo struct {
	BootDevice    string // e.g. /dev/sda  (where OS runs from)
	ExcludedPaths []string
}

// GetSafetyInfo returns devices that should be protected from accidental backup/restore
func GetSafetyInfo() *SafetyInfo {
	info := &SafetyInfo{}

	// Find boot/root device
	if dev := findRootDevice(); dev != "" {
		info.BootDevice = dev
		info.ExcludedPaths = append(info.ExcludedPaths, dev)
	}

	return info
}

// IsSafeBackupSource returns true if the device is safe to use as backup source.
// It blocks the current boot/root device to prevent accidental self-backup of live system.
func IsSafeBackupSource(devicePath string, safety *SafetyInfo) (safe bool, reason string) {
	if safety == nil {
		return true, ""
	}
	base := parentDisk(devicePath)
	bootBase := parentDisk(safety.BootDevice)

	if base != "" && base == bootBase {
		return false, "Systemlaufwerk (aktives OS)"
	}
	return true, ""
}

// IsSafeRestoreTarget returns true if device is safe to restore onto.
// Blocks: current boot disk, and device containing the backup image file.
func IsSafeRestoreTarget(devicePath, imageFilePath string, safety *SafetyInfo) (safe bool, reason string) {
	if safety == nil {
		return true, ""
	}

	base := parentDisk(devicePath)
	bootBase := parentDisk(safety.BootDevice)

	// Block restoring onto the currently booted disk
	if base != "" && base == bootBase {
		return false, "Aktives Systemlaufwerk – Restore würde laufendes OS zerstören!"
	}

	// Block restoring onto the disk that holds the backup image
	if imageFilePath != "" {
		imgDisk := diskForPath(imageFilePath)
		if imgDisk != "" && imgDisk == base {
			return false, "Backup-Datei liegt auf diesem Gerät – würde Quelle überschreiben!"
		}
	}

	return true, ""
}

// FilterSafeDisks returns only disks that are safe backup sources,
// annotating unsafe ones with a reason.
type AnnotatedDisk struct {
	Disk   DiskInfo
	Safe   bool
	Reason string
}

func AnnotateDisks(disks []DiskInfo, safety *SafetyInfo) []AnnotatedDisk {
	out := make([]AnnotatedDisk, 0, len(disks))
	for _, d := range disks {
		safe, reason := IsSafeBackupSource(d.Path, safety)
		out = append(out, AnnotatedDisk{Disk: d, Safe: safe, Reason: reason})
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

// findRootDevice returns the block device backing the / mount point
func findRootDevice() string {
	// Try reading /proc/mounts
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		dev, mount := fields[0], fields[1]
		if mount == "/" && strings.HasPrefix(dev, "/dev/") {
			return parentDisk(dev)
		}
	}
	return ""
}

// parentDisk returns the parent disk of a partition path, e.g. /dev/sda1 → /dev/sda
func parentDisk(path string) string {
	if path == "" {
		return ""
	}
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	base := filepath.Base(resolved)

	// Use lsblk to find the parent disk
	out, err := exec.Command("lsblk", "-no", "PKNAME", resolved).Output()
	if err == nil {
		parent := strings.TrimSpace(string(out))
		if parent != "" {
			return "/dev/" + parent
		}
	}

	// Fallback: strip trailing digits (sda1 → sda, nvme0n1p1 → nvme0n1)
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] < '0' || base[i] > '9' {
			stripped := base[:i+1]
			// nvme special: strip trailing 'p' before partition number
			stripped = strings.TrimRight(stripped, "p")
			if stripped != "" {
				return "/dev/" + stripped
			}
			break
		}
	}
	return resolved
}

// diskForPath returns the block device that contains the given file path
func diskForPath(path string) string {
	dir := filepath.Dir(path)

	// Walk up to find the mount point, then match to device
	out, err := exec.Command("df", "--output=source", dir).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return ""
	}
	dev := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(dev, "/dev/") {
		return ""
	}
	return parentDisk(dev)
}
