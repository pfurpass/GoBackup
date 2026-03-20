package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/backup-tool/core"
)

// DiskScreen displays all detected block devices
type DiskScreen struct {
	app   *Application
	table *widget.Table
	disks []core.DiskInfo // flat list
	info  *widget.Label
}

func NewDiskScreen(app *Application) *DiskScreen {
	return &DiskScreen{app: app}
}

// Build returns the complete disk screen widget
func (s *DiskScreen) Build() fyne.CanvasObject {
	// Column headers
	headers := []string{"Gerät", "Modell", "Typ", "Dateisystem", "Gemountet", "Größe"}

	s.info = widget.NewLabel("Disks werden geladen…")
	s.info.Wrapping = fyne.TextWrapWord

	s.table = widget.NewTable(
		// Size func
		func() (int, int) { return len(s.disks) + 1, len(headers) },

		// Create cell
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},

		// Update cell
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			lbl := cell.(*widget.Label)
			if id.Row == 0 {
				lbl.SetText(headers[id.Col])
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			disk := s.disks[id.Row-1]
			switch id.Col {
			case 0:
				prefix := ""
				if disk.DevType == "part" {
					prefix = "  > "
				}
				lbl.SetText(prefix + disk.Path)
			case 1:
				if disk.Model != "" {
					lbl.SetText(disk.Model)
				} else {
					lbl.SetText("—")
				}
			case 2:
				if disk.DevType == "disk" {
					lbl.SetText(disk.DiskClass)
				} else {
					lbl.SetText("Partition")
				}
			case 3:
				if disk.FSType != "" {
					lbl.SetText(disk.FSType)
				} else {
					lbl.SetText("—")
				}
			case 4:
				if disk.MountPoint != "" {
					lbl.SetText(disk.MountPoint)
				} else {
					lbl.SetText("nicht gemountet")
				}
			case 5:
				lbl.SetText(disk.SizeHuman)
			}
		},
	)

	// Set column widths
	s.table.SetColumnWidth(0, 160)
	s.table.SetColumnWidth(1, 220)
	s.table.SetColumnWidth(2, 90)
	s.table.SetColumnWidth(3, 100)
	s.table.SetColumnWidth(4, 170)
	s.table.SetColumnWidth(5, 90)

	// Row selection → update info bar and set shared selectedDisk
	s.table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 || id.Row > len(s.disks) {
			return
		}
		disk := s.disks[id.Row-1]
		s.app.selectedDisk = &disk
		s.info.SetText(fmt.Sprintf(
			"Ausgewählt: %s   Modell: %s   Größe: %s   Typ: %s",
			disk.Path, disk.Model, disk.SizeHuman, disk.DiskClass,
		))
	}

	refreshBtn := widget.NewButtonWithIcon("Aktualisieren", theme.ViewRefreshIcon(), func() {
		s.refresh()
	})
	selectBackupBtn := widget.NewButtonWithIcon("→ Backup", theme.UploadIcon(), func() {
		s.app.SwitchToTab(1)
	})

	toolbar := container.NewHBox(refreshBtn, widget.NewSeparator(), selectBackupBtn)
	statusBar := container.NewBorder(nil, nil,
		widget.NewIcon(theme.InfoIcon()), nil, s.info)

	layout := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Erkannte Laufwerke", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			toolbar,
			widget.NewSeparator(),
		),
		container.NewVBox(widget.NewSeparator(), statusBar),
		nil, nil,
		s.table,
	)

	// Initial load
	go s.refresh()

	return layout
}

func (s *DiskScreen) refresh() {
	disks, err := core.ListDisks()
	if err != nil {
		s.info.SetText("Fehler beim Laden: " + err.Error())
		return
	}
	s.disks = core.FlatList(disks)
	s.table.Refresh()
	s.info.SetText(fmt.Sprintf("%d Laufwerk(e) gefunden", len(disks)))
}
