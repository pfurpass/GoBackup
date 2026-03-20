package ui

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/backup-tool/core"
	"github.com/user/backup-tool/utils"
)

const appID = "de.gobackup.tool"
const appTitle = "GoBackup – Disk Backup & Restore"

// Application is the root UI controller
type Application struct {
	fyneApp fyne.App
	window  fyne.Window
	logger  *utils.Logger

	// Shared state across screens
	selectedDisk   *core.DiskInfo
	backupFilePath string

	// Child screens
	diskScreen     *DiskScreen
	backupScreen   *BackupScreen
	restoreScreen  *RestoreScreen
	progressScreen *ProgressScreen

	tabs *container.AppTabs
}

// NewApplication creates and wires all UI components
func NewApplication() *Application {
	a := &Application{}

	a.fyneApp = fyneApp.NewWithID(appID)
	a.fyneApp.Settings().SetTheme(theme.DarkTheme())

	a.window = a.fyneApp.NewWindow(appTitle)
	a.window.Resize(fyne.NewSize(960, 680))
	a.window.SetMaster()

	// Logger
	logPath := filepath.Join(os.TempDir(), "gobackup.log")
	logger, err := utils.NewLogger(logPath, nil)
	if err != nil {
		logger = utils.NewConsoleLogger()
	}
	a.logger = logger

	// Build screens
	a.progressScreen = NewProgressScreen(a)
	a.diskScreen = NewDiskScreen(a)
	a.backupScreen = NewBackupScreen(a)
	a.restoreScreen = NewRestoreScreen(a)

	// Tab navigation
	a.tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("Disks", theme.StorageIcon(), a.diskScreen.Build()),
		container.NewTabItemWithIcon("Backup", theme.UploadIcon(), a.backupScreen.Build()),
		container.NewTabItemWithIcon("Restore", theme.DownloadIcon(), a.restoreScreen.Build()),
		container.NewTabItemWithIcon("Log", theme.DocumentIcon(), a.progressScreen.Build()),
	)
	a.tabs.SetTabLocation(container.TabLocationLeading)

	// Header bar
	header := buildHeader()
	content := container.NewBorder(header, nil, nil, nil, a.tabs)

	a.window.SetContent(content)
	return a
}

// Run starts the Fyne event loop
func (a *Application) Run() {
	a.window.ShowAndRun()
}

// SwitchToTab navigates to a named tab (0=Disks, 1=Backup, 2=Restore, 3=Log)
func (a *Application) SwitchToTab(idx int) {
	if idx >= 0 && idx < len(a.tabs.Items) {
		a.tabs.SelectIndex(idx)
	}
}

// ── private ───────────────────────────────────────────────────────────────────

func buildHeader() fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"  🔷 GoBackup  |  Block-Level Disk Backup für Linux",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
	)
	version := widget.NewLabel("v1.0.0")
	version.Alignment = fyne.TextAlignTrailing
	return container.NewBorder(nil, nil, title, version,
		widget.NewSeparator())
}
