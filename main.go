package main

import (
	"fmt"
	"os"

	"github.com/user/backup-tool/ui"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "⚠️  Warnung: Kein Root-Zugriff. Disk-Operationen könnten fehlschlagen.")
		fmt.Fprintln(os.Stderr, "   Starte mit: sudo ./backup-tool")
	}

	// Ensure DISPLAY is set for X11 when running as root via sudo
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		// Try common display values
		os.Setenv("DISPLAY", ":0")
	}

	// Fyne emits D-Bus warnings on headless/minimal systems – these are non-fatal.
	// Set DBUS_SESSION_BUS_ADDRESS to suppress the reconnect noise when not needed.
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", "autolaunch:")
	}

	application := ui.NewApplication()
	application.Run()
}
