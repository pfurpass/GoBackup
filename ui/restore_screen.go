package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/backup-tool/core"
)

// RestoreScreen provides the restore configuration UI
type RestoreScreen struct {
	app *Application

	// Source image
	imageEntry  *widget.Entry
	metaLabel   *widget.Label

	// Destination
	diskSelect *widget.Select
	diskPaths  []string

	// Options
	passEntry *widget.Entry
	passRow   *fyne.Container

	warnLabel *widget.Label
	startBtn  *widget.Button
}

func NewRestoreScreen(app *Application) *RestoreScreen {
	return &RestoreScreen{app: app}
}

func (s *RestoreScreen) Build() fyne.CanvasObject {
	// ── Image selection ───────────────────────────────────────────────────────
	imgTitle := widget.NewLabelWithStyle("1. Backup-Image auswählen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	s.imageEntry = widget.NewEntry()
	s.imageEntry.SetPlaceHolder("/mnt/backup/backup.img")
	s.imageEntry.OnChanged = func(path string) {
		s.loadMeta(path)
	}

	browseBtn := widget.NewButtonWithIcon("Öffnen", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			uc.Close()
			s.imageEntry.SetText(uc.URI().Path())
		}, s.app.window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".img"}))
		fd.Show()
	})

	imgRow := container.NewBorder(nil, nil, nil, browseBtn, s.imageEntry)

	s.metaLabel = widget.NewLabel("Kein Image geladen")
	s.metaLabel.Wrapping = fyne.TextWrapWord
	metaCard := widget.NewCard("Image-Informationen", "", s.metaLabel)

	// ── Target disk ───────────────────────────────────────────────────────────
	diskTitle := widget.NewLabelWithStyle("2. Zielgerät wählen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	s.diskSelect = widget.NewSelect([]string{"Lade…"}, func(_ string) {})
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go s.loadDisks()
	})
	diskRow := container.NewBorder(nil, nil, nil, refreshBtn, s.diskSelect)

	// ── Password (conditionally shown) ────────────────────────────────────────
	s.passEntry = widget.NewPasswordEntry()
	s.passEntry.SetPlaceHolder("Entschlüsselungs-Passwort…")
	s.passRow = container.NewVBox(
		widget.NewLabel("Passwort (für verschlüsselte Backups):"),
		s.passEntry,
	)
	s.passRow.Hide()

	// ── Warning banner ────────────────────────────────────────────────────────
	s.warnLabel = widget.NewLabelWithStyle(
		"⚠️  ACHTUNG: Alle Daten auf dem Zielgerät werden unwiderruflich überschrieben!",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// ── Start button ──────────────────────────────────────────────────────────
	s.startBtn = widget.NewButtonWithIcon("  Restore starten", theme.DownloadIcon(), func() {
		s.confirmAndStart()
	})
	s.startBtn.Importance = widget.DangerImportance

	// ── Layout ───────────────────────────────────────────────────────────────
	form := container.NewVBox(
		imgTitle,
		imgRow,
		metaCard,
		widget.NewSeparator(),
		diskTitle,
		diskRow,
		widget.NewSeparator(),
		s.passRow,
		widget.NewSeparator(),
		s.warnLabel,
		s.startBtn,
	)

	padded := container.NewPadded(container.NewVScroll(form))

	go s.loadDisks()
	return padded
}

func (s *RestoreScreen) loadDisks() {
	disks, err := core.ListDisks()
	if err != nil {
		s.diskSelect.Options = []string{"Fehler: " + err.Error()}
		s.diskSelect.Refresh()
		return
	}
	flat := core.FlatList(disks)
	s.diskPaths = make([]string, 0, len(flat))
	labels := make([]string, 0, len(flat))
	for _, d := range flat {
		s.diskPaths = append(s.diskPaths, d.Path)
		prefix := ""
		if d.DevType == "part" {
			prefix = "  > "
		}
		labels = append(labels, fmt.Sprintf("%s%s  [%s]", prefix, d.Path, d.SizeHuman))
	}
	s.diskSelect.Options = labels
	if len(labels) > 0 {
		s.diskSelect.SetSelectedIndex(0)
	}
	s.diskSelect.Refresh()
}

func (s *RestoreScreen) loadMeta(path string) {
	if path == "" {
		s.metaLabel.SetText("Kein Image angegeben")
		s.passRow.Hide()
		return
	}

	meta, err := core.ReadMeta(path)
	if err != nil {
		s.metaLabel.SetText("Fehler: " + err.Error())
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ursprüngliches Gerät: %s\n", meta.DevicePath))
	sb.WriteString(fmt.Sprintf("Datengröße:           %s\n", core.FormatSize(meta.DeviceSize)))
	sb.WriteString(fmt.Sprintf("Chunk-Größe:          %s\n", core.FormatSize(uint64(meta.ChunkSize))))

	flags := []string{}
	if meta.Compressed {
		flags = append(flags, "zstd-komprimiert")
	}
	if meta.Encrypted {
		flags = append(flags, "AES-256-verschlüsselt")
		s.passRow.Show()
	} else {
		s.passRow.Hide()
	}
	if len(flags) == 0 {
		flags = append(flags, "keine")
	}
	sb.WriteString(fmt.Sprintf("Optionen:             %s\n", strings.Join(flags, ", ")))

	s.metaLabel.SetText(sb.String())
	s.app.backupFilePath = path
}

func (s *RestoreScreen) selectedDisk() string {
	idx := s.diskSelect.SelectedIndex()
	if idx < 0 || idx >= len(s.diskPaths) {
		return ""
	}
	return s.diskPaths[idx]
}

func (s *RestoreScreen) confirmAndStart() {
	imgPath := s.imageEntry.Text
	target := s.selectedDisk()

	if imgPath == "" {
		dialog.ShowError(fmt.Errorf("Kein Backup-Image angegeben"), s.app.window)
		return
	}
	if target == "" {
		dialog.ShowError(fmt.Errorf("Kein Zielgerät ausgewählt"), s.app.window)
		return
	}

	// ── Safety check ──────────────────────────────────────────────────────────
	safety := core.GetSafetyInfo()
	if ok, reason := core.IsSafeRestoreTarget(target, imgPath, safety); !ok {
		dialog.ShowError(fmt.Errorf(
			"⛔ Sicherheitssperre!\n\nZiel: %s\nGrund: %s\n\nWähle ein anderes Zielgerät.", target, reason,
		), s.app.window)
		return
	}

	warningMsg := fmt.Sprintf(
		"⚠️  LETZTE WARNUNG!\n\n"+
			"Image:   %s\n"+
			"Ziel:    %s\n\n"+
			"Alle Daten auf dem Zielgerät werden DAUERHAFT gelöscht!\n\n"+
			"Sind Sie ABSOLUT sicher?",
		imgPath, target,
	)

	dialog.ShowConfirm("Restore bestätigen", warningMsg, func(confirmed bool) {
		if !confirmed {
			return
		}

		opts := core.RestoreOptions{
			Source:      imgPath,
			Destination: target,
			Password:    s.passEntry.Text,
		}

		s.app.SwitchToTab(3)
		s.app.progressScreen.StartRestore(opts)
	}, s.app.window)
}
