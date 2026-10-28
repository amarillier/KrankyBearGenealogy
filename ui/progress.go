package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

// runWithProgressBar is like runWithProgress but shows a determinate progress
// bar. work calls report(done, status) as it goes; report is safe to call from
// the background goroutine.
func runWithProgressBar(w fyne.Window, title string, total int, work func(report func(done int, status string)) error, onDone func(error)) {
	bar := widget.NewProgressBar()
	bar.Max = float64(max(total, 1))
	status := widget.NewLabel("Starting...")
	content := container.NewVBox(status, bar)
	progress := dialog.NewCustomWithoutButtons(title, content, w)
	progress.Resize(fyne.NewSize(420, progress.MinSize().Height))
	progress.Show()

	go func() {
		err := work(func(done int, msg string) {
			fyne.Do(func() {
				bar.SetValue(float64(done))
				status.SetText(msg)
			})
		})
		fyne.Do(func() {
			progress.Hide()
			onDone(err)
		})
	}()
}
