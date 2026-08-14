package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// Track the audit log window so a second click brings the existing one to
// front instead of opening a duplicate.
var auditLogWindow fyne.Window

// showAuditLogWindow shows a read-only, chronological view of every
// tracked change (person edits/deletes, relationship adds/deletes) -
// persistent across app restarts, unlike the in-memory Undo/Redo buffer.
func showAuditLogWindow(parentWindow fyne.Window, s *store.Store) {
	if auditLogWindow != nil {
		auditLogWindow.RequestFocus()
		auditLogWindow.Show()
		return
	}

	auditLogWindow = fyne.CurrentApp().NewWindow("Audit Log")

	entries, err := s.GetAllAuditLogEntries()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load audit log: %w", err), parentWindow)
		auditLogWindow = nil
		return
	}

	content := container.NewVBox()

	header := widget.NewLabelWithStyle(
		fmt.Sprintf("Audit Log (%d entries)", len(entries)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(header)
	content.Add(widget.NewLabel("Persistent record of edits, deletions, and relationship changes."))
	content.Add(widget.NewSeparator())

	if len(entries) == 0 {
		content.Add(widget.NewLabel("No changes tracked yet."))
	} else {
		for _, entry := range entries {
			content.Add(makeAuditLogCard(entry))
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 700))
	auditLogWindow.SetContent(scroll)

	auditLogWindow.Resize(fyne.NewSize(800, 800))
	auditLogWindow.SetOnClosed(func() {
		auditLogWindow = nil
	})
	auditLogWindow.Show()
}

// makeAuditLogCard creates a read-only card for a single audit log entry.
func makeAuditLogCard(entry store.AuditLogEntry) *fyne.Container {
	timeLabel := widget.NewLabelWithStyle(
		entry.Timestamp.Local().Format("January 2, 2006 at 3:04 PM"),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	descLabel := widget.NewLabel(entry.Description)
	descLabel.Wrapping = fyne.TextWrapWord

	return container.NewVBox(timeLabel, descLabel)
}
