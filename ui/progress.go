package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// runWithProgress shows an indeterminate progress dialog, runs work on a
// background goroutine so the UI stays responsive, then hands the result
// back to onDone on the UI goroutine once work finishes.
func runWithProgress(w fyne.Window, title, message string, work func() error, onDone func(error)) {
	progress := dialog.NewCustomWithoutButtons(title, widget.NewLabel(message), w)
	progress.Show()

	go func() {
		err := work()
		fyne.Do(func() {
			progress.Hide()
			onDone(err)
		})
	}()
}
