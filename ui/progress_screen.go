package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/backup-tool/core"
	"github.com/user/backup-tool/utils"
)

// ProgressScreen shows live progress of backup/restore operations
type ProgressScreen struct {
	app *Application

	// Status widgets
	operationLabel *widget.Label
	statusLabel    *widget.Label
	progressBar    *widget.ProgressBar
	speedLabel     *widget.Label
	etaLabel       *widget.Label
	doneLabel      *widget.Label

	// Log
	logText  *widget.TextGrid
	logLines []string
	logMu    sync.Mutex

	cancelCh chan struct{}
	running  bool
}

func NewProgressScreen(app *Application) *ProgressScreen {
	return &ProgressScreen{app: app}
}

// Build constructs the progress screen widget tree
func (s *ProgressScreen) Build() fyne.CanvasObject {
	s.operationLabel = widget.NewLabelWithStyle(
		"Bereit", fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Italic: true},
	)

	s.progressBar = widget.NewProgressBar()
	s.progressBar.Min = 0
	s.progressBar.Max = 1

	s.statusLabel = widget.NewLabel("–")
	s.statusLabel.Alignment = fyne.TextAlignCenter
	s.statusLabel.Wrapping = fyne.TextWrapWord

	speedEtaRow := container.NewGridWithColumns(3,
		newStat("Geschwindigkeit", &s.speedLabel),
		newStat("Fortschritt", &s.doneLabel),
		newStat("Verbleibend", &s.etaLabel),
	)

	s.logText = widget.NewTextGrid()
	s.logText.SetText("Log-Ausgabe erscheint hier…\n")

	logScroll := container.NewScroll(s.logText)
	logScroll.SetMinSize(fyne.NewSize(400, 220))

	logCard := widget.NewCard("Live-Log", "", logScroll)

	layout := container.NewVBox(
		container.NewPadded(s.operationLabel),
		container.NewPadded(s.progressBar),
		container.NewPadded(s.statusLabel),
		container.NewPadded(speedEtaRow),
		widget.NewSeparator(),
		container.NewPadded(logCard),
	)

	return container.NewPadded(container.NewVScroll(layout))
}

// StartBackup launches a backup operation and updates the UI
func (s *ProgressScreen) StartBackup(opts core.BackupOptions) {
	if s.running {
		return
	}
	s.reset("🔷 Backup läuft…")

	logger, _ := utils.NewLogger(
		fmt.Sprintf("/tmp/gobackup_%d.log", time.Now().Unix()), nil,
	)

	engine := core.NewBackupEngine(opts, logger)
	ch := make(chan core.Progress, 32)

	s.running = true
	go engine.Run(ch)
	go s.consumeProgress(ch, "Backup")
}

// StartRestore launches a restore operation and updates the UI
func (s *ProgressScreen) StartRestore(opts core.RestoreOptions) {
	if s.running {
		return
	}
	s.reset("🔄 Restore läuft…")

	logger, _ := utils.NewLogger(
		fmt.Sprintf("/tmp/gobackup_%d.log", time.Now().Unix()), nil,
	)

	engine := core.NewRestoreEngine(opts, logger)
	ch := make(chan core.Progress, 32)

	s.running = true
	go engine.Run(ch)
	go s.consumeProgress(ch, "Restore")
}

// ── private ───────────────────────────────────────────────────────────────────

func (s *ProgressScreen) reset(title string) {
	s.operationLabel.SetText(title)
	s.progressBar.SetValue(0)
	s.statusLabel.SetText("Initialisiere…")
	s.speedLabel.SetText("— MB/s")
	s.etaLabel.SetText("–")
	s.doneLabel.SetText("0 B")
	s.logMu.Lock()
	s.logLines = nil
	s.logMu.Unlock()
	s.logText.SetText("")
}

func (s *ProgressScreen) consumeProgress(ch <-chan core.Progress, op string) {
	defer func() { s.running = false }()

	for p := range ch {
		// Update log
		if p.Message != "" {
			s.appendLog(p.Message)
		}

		// Update progress bar
		if p.BytesTotal > 0 {
			ratio := float64(p.BytesDone) / float64(p.BytesTotal)
			s.progressBar.SetValue(math.Min(ratio, 1.0))
			s.doneLabel.SetText(fmt.Sprintf("%s / %s",
				core.FormatSize(p.BytesDone), core.FormatSize(p.BytesTotal)))
		}

		// Speed
		if p.SpeedBPS > 0 {
			s.speedLabel.SetText(fmt.Sprintf("%.1f MB/s", p.SpeedBPS/1024/1024))
		}

		// ETA
		if p.ETA > 0 {
			s.etaLabel.SetText(formatDuration(p.ETA))
		}

		// Status line
		if p.Message != "" {
			s.statusLabel.SetText(p.Message)
		}

		// Done / Error
		if p.Done || p.Error != nil {
			if p.Error != nil {
				s.operationLabel.SetText("❌ Fehler!")
				s.statusLabel.SetText("FEHLER: " + p.Error.Error())
				s.appendLog("FEHLER: " + p.Error.Error())
				s.progressBar.SetValue(0)
			} else {
				s.operationLabel.SetText(fmt.Sprintf("✅ %s abgeschlossen", op))
				s.progressBar.SetValue(1)
			}
			return
		}
	}
}

func (s *ProgressScreen) appendLog(msg string) {
	s.logMu.Lock()
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", ts, msg)
	s.logLines = append(s.logLines, line)
	// Keep last 500 lines
	if len(s.logLines) > 500 {
		s.logLines = s.logLines[len(s.logLines)-500:]
	}
	text := strings.Join(s.logLines, "\n") + "\n"
	s.logMu.Unlock()

	s.logText.SetText(text)
}

// ── widget helpers ────────────────────────────────────────────────────────────

// newStat creates a label+value pair card for the stats row
func newStat(title string, lbl **widget.Label) fyne.CanvasObject {
	*lbl = widget.NewLabelWithStyle("–", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	caption := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	return widget.NewCard("", "", container.NewVBox(
		container.NewCenter(widget.NewIcon(theme.InfoIcon())),
		container.NewCenter(*lbl),
		container.NewCenter(caption),
	))
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm %02ds", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, sec)
	}
	return fmt.Sprintf("%ds", sec)
}
