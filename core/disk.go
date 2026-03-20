package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DiskInfo holds metadata about a block device or partition
type DiskInfo struct {
	Name       string
	Path       string
	Size       uint64
	SizeHuman  string
	DevType    string // "disk" | "part" | "rom"
	FSType     string // "ext4", "ntfs", …
	MountPoint string
	Model      string
	DiskClass  string // "SSD/NVMe" | "HDD"
	Rotational bool
	Children   []DiskInfo
}

// lsblkRoot is the JSON envelope returned by lsblk -J
type lsblkRoot struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

// flexSize accepts both "12345" (string) and 12345 (number) from lsblk
type flexSize struct{ v uint64 }

func (f *flexSize) UnmarshalJSON(b []byte) error {
	// Try number first
	var n uint64
	if err := json.Unmarshal(b, &n); err == nil {
		f.v = n
		return nil
	}
	// Try quoted string
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return err
	}
	f.v = n
	return nil
}

// flexBool accepts both true/false (bool) and "0"/"1" (string) from lsblk
type flexBool struct{ v bool }

func (f *flexBool) UnmarshalJSON(b []byte) error {
	// Try bool first (newer lsblk)
	var bv bool
	if err := json.Unmarshal(b, &bv); err == nil {
		f.v = bv
		return nil
	}
	// Try string "0" / "1"
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	f.v = strings.TrimSpace(s) == "1"
	return nil
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Size       flexSize      `json:"size"`
	Type       string        `json:"type"`
	FSType     *string       `json:"fstype"`
	MountPoint *string       `json:"mountpoint"`
	Model      *string       `json:"model"`
	Rota       flexBool      `json:"rota"`
	Children   []lsblkDevice `json:"children"`
}

// ListDisks returns all physical block devices (no loop devices)
func ListDisks() ([]DiskInfo, error) {
	cmd := exec.Command("lsblk", "-J", "-b", "-o",
		"NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,ROTA")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk failed: %w", err)
	}

	var root lsblkRoot
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, fmt.Errorf("lsblk JSON parse: %w", err)
	}

	var disks []DiskInfo
	for _, bd := range root.BlockDevices {
		if strings.HasPrefix(bd.Name, "loop") {
			continue
		}
		disks = append(disks, convertDevice(bd))
	}
	return disks, nil
}

func convertDevice(bd lsblkDevice) DiskInfo {
	size := bd.Size.v
	rotational := bd.Rota.v

	class := "SSD/NVMe"
	if rotational {
		class = "HDD"
	}

	info := DiskInfo{
		Name:       bd.Name,
		Path:       "/dev/" + bd.Name,
		Size:       size,
		SizeHuman:  FormatSize(size),
		DevType:    bd.Type,
		Rotational: rotational,
		DiskClass:  class,
	}
	if bd.FSType != nil {
		info.FSType = *bd.FSType
	}
	if bd.MountPoint != nil {
		info.MountPoint = *bd.MountPoint
	}
	if bd.Model != nil {
		info.Model = strings.TrimSpace(*bd.Model)
	}

	for _, child := range bd.Children {
		info.Children = append(info.Children, convertDevice(child))
	}
	return info
}

// FlatList returns disks and their partitions in a flat slice for display
func FlatList(disks []DiskInfo) []DiskInfo {
	var out []DiskInfo
	for _, d := range disks {
		out = append(out, d)
		for _, p := range d.Children {
			out = append(out, p)
		}
	}
	return out
}

// FormatSize converts bytes to human-readable string (KiB, MiB, GiB, TiB)
func FormatSize(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
