package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/backup-tool/core"
)

// BackupScreen provides the backup configuration and start UI
type BackupScreen struct {
	app *Application

	// Source
	diskSelect   *widget.Select
	diskPaths    []string

	// Destination
	destEntry *widget.Entry

	// Options
	compressCheck *widget.Check
	encryptCheck  *widget.Check
	passEntry     *widget.Entry
	passRow       *fyne.Container

	// Summary
	summaryLabel *widget.Label

	startBtn *widget.Button
}

func NewBackupScreen(app *Application) *BackupScreen {
	return &BackupScreen{app: app}
}

func (s *BackupScreen) Build() fyne.CanvasObject {
	// ── Source disk selection ─────────────────────────────────────────────────
	sourceTitle := widget.NewLabelWithStyle("1. Quellgerät wählen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	s.diskSelect = widget.NewSelect([]string{"Lade…"}, func(val string) {
		s.updateSummary()
	})
	refreshDisksBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go s.loadDisks()
	})

	sourceRow := container.NewBorder(nil, nil, nil, refreshDisksBtn, s.diskSelect)

	// ── Destination ───────────────────────────────────────────────────────────
	destTitle := widget.NewLabelWithStyle("2. Backup-Ziel", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	s.destEntry = widget.NewEntry()
	s.destEntry.SetPlaceHolder("/mnt/backup/mein-backup.img")
	s.destEntry.OnChanged = func(_ string) { s.updateSummary() }

	browseBtn := widget.NewButtonWithIcon("Durchsuchen", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			uc.Close()
			s.destEntry.SetText(uc.URI().Path())
		}, s.app.window)
		fd.SetFileName(defaultFilename())
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".img"}))
		fd.Show()
	})

	destRow := container.NewBorder(nil, nil, nil, browseBtn, s.destEntry)

	// ── Options ───────────────────────────────────────────────────────────────
	optsTitle := widget.NewLabelWithStyle("3. Optionen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	s.compressCheck = widget.NewCheck("zstd Kompression (spart Speicherplatz)", func(v bool) {
		s.updateSummary()
	})

	s.passEntry = widget.NewPasswordEntry()
	s.passEntry.SetPlaceHolder("Passwort eingeben…")

	s.passRow = container.NewVBox(
		widget.NewLabel("Passwort (AES-256-GCM):"),
		s.passEntry,
	)
	s.passRow.Hide()

	s.encryptCheck = widget.NewCheck("AES-256-GCM Verschlüsselung", func(v bool) {
		if v {
			s.passRow.Show()
		} else {
			s.passRow.Hide()
		}
		s.updateSummary()
	})

	optionsBox := container.NewVBox(
		s.compressCheck,
		s.encryptCheck,
		s.passRow,
	)

	// ── Summary ───────────────────────────────────────────────────────────────
	s.summaryLabel = widget.NewLabel("Bitte Quellgerät und Ziel auswählen.")
	s.summaryLabel.Wrapping = fyne.TextWrapWord

	// Set defaults AFTER all widgets exist to avoid nil callbacks
	s.compressCheck.SetChecked(true)

	summaryCard := widget.NewCard("Zusammenfassung", "", s.summaryLabel)

	// ── Start button ──────────────────────────────────────────────────────────
	s.startBtn = widget.NewButtonWithIcon("  Backup starten", theme.UploadIcon(), func() {
		s.startBackup()
	})
	s.startBtn.Importance = widget.HighImportance

	// ── Layout ───────────────────────────────────────────────────────────────
	form := container.NewVBox(
		sourceTitle,
		sourceRow,
		widget.NewSeparator(),
		destTitle,
		destRow,
		widget.NewSeparator(),
		optsTitle,
		optionsBox,
		widget.NewSeparator(),
		summaryCard,
		s.startBtn,
	)

	padded := container.NewPadded(container.NewVScroll(form))

	// Load disks on first build
	go s.loadDisks()

	return padded
}

func (s *BackupScreen) loadDisks() {
	disks, err := core.ListDisks()
	if err != nil {
		s.diskSelect.Options = []string{"Fehler beim Laden"}
		s.diskSelect.Refresh()
		return
	}

	flat := core.FlatList(disks)
	s.diskPaths = make([]string, 0, len(flat))
	labels := make([]string, 0, len(flat))
	for _, d := range flat {
		s.diskPaths = append(s.diskPaths, d.Path)
		label := fmt.Sprintf("%s  [%s]  %s", d.Path, d.SizeHuman, d.Model)
		if d.DevType == "part" {
				label = "  > " + label
			}
		labels = append(labels, label)
	}

	s.diskSelect.Options = labels
	if len(labels) > 0 {
		s.diskSelect.SetSelectedIndex(0)
	}
	s.diskSelect.Refresh()
	s.updateSummary()
}

func (s *BackupScreen) selectedPath() string {
	idx := s.diskSelect.SelectedIndex()
	if idx < 0 || idx >= len(s.diskPaths) {
		return ""
	}
	return s.diskPaths[idx]
}

func (s *BackupScreen) updateSummary() {
	if s.summaryLabel == nil || s.destEntry == nil {
		return
	}
	src := s.selectedPath()
	dst := s.destEntry.Text
	comp := s.compressCheck.Checked
	enc := s.encryptCheck.Checked

	opts := ""
	if comp {
		opts += "zstd "
	}
	if enc {
		opts += "AES-256 "
	}
	if opts == "" {
		opts = "keine"
	}

	s.summaryLabel.SetText(fmt.Sprintf(
		"Quelle:    %s\nZiel:      %s\nOptionen: %s",
		orEmpty(src, "—"), orEmpty(dst, "—"), opts,
	))
}

func (s *BackupScreen) startBackup() {
	src := s.selectedPath()
	dst := s.destEntry.Text

	if src == "" {
		dialog.ShowError(fmt.Errorf("Kein Quellgerät ausgewählt"), s.app.window)
		return
	}
	if dst == "" {
		dialog.ShowError(fmt.Errorf("Kein Zieldatei angegeben"), s.app.window)
		return
	}
	if s.encryptCheck.Checked && s.passEntry.Text == "" {
		dialog.ShowError(fmt.Errorf("Bitte Passwort eingeben"), s.app.window)
		return
	}

	// ── Safety check ──────────────────────────────────────────────────────────
	safety := core.GetSafetyInfo()
	if ok, reason := core.IsSafeBackupSource(src, safety); !ok {
		dialog.ShowError(fmt.Errorf(
			"⛔ Sicherheitssperre!\n\nGerät: %s\nGrund: %s\n\nWähle ein anderes Quellgerät.", src, reason,
		), s.app.window)
		return
	}

	opts := core.BackupOptions{
		Source:      src,
		Destination: dst,
		Compress:    s.compressCheck.Checked,
		Encrypt:     s.encryptCheck.Checked,
		Password:    s.passEntry.Text,
	}

	// Switch to progress screen
	s.app.SwitchToTab(3)
	s.app.progressScreen.StartBackup(opts)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func defaultFilename() string {
	ts := time.Now().Format("2006-01-02_15-04")
	return filepath.Base(fmt.Sprintf("backup_%s.img", ts))
}

func orEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}


