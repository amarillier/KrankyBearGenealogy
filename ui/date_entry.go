package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// DateEntry is an enhanced entry widget for date input with validation and calendar picker
type DateEntry struct {
	Entry          *widget.Entry
	ValidationIcon *widget.Label
	CalendarButton *widget.Button
	Container      *fyne.Container
	window         fyne.Window
}

// NewDateEntry creates a new date entry widget with validation and calendar picker
func NewDateEntry(window fyne.Window) *DateEntry {
	de := &DateEntry{
		Entry:          widget.NewEntry(),
		ValidationIcon: widget.NewLabel(""),
		window:         window,
	}

	de.Entry.SetPlaceHolder("YYYY-MM-DD, DD Mon YYYY, or click 📅")

	// Create calendar button
	de.CalendarButton = widget.NewButton("📅", func() {
		de.showCalendarPicker()
	})

	// Set up real-time validation
	de.Entry.OnChanged = func(text string) {
		de.updateValidation(text)
	}

	// Layout: [Entry] [ValidationIcon] [📅]
	de.Container = container.NewBorder(nil, nil, nil,
		container.NewHBox(de.ValidationIcon, de.CalendarButton),
		de.Entry)

	return de
}

// updateValidation updates the validation icon based on the current text
func (de *DateEntry) updateValidation(text string) {
	if text == "" {
		de.ValidationIcon.SetText("")
		return
	}

	result := ParseGenealogyDate(text)
	if result.Valid {
		// Show checkmark for valid dates
		icon := "✓"
		if result.IsEstimated {
			icon = "✓~" // Checkmark with tilde for estimated
		} else if result.IsBefore {
			icon = "✓<" // Checkmark with less-than for before
		} else if result.IsAfter {
			icon = "✓>" // Checkmark with greater-than for after
		}
		de.ValidationIcon.SetText(icon)
	} else {
		// Show warning for invalid dates
		de.ValidationIcon.SetText("⚠")
	}
}

// showCalendarPicker shows a calendar picker dialog
func (de *DateEntry) showCalendarPicker() {
	// Create a calendar picker
	var selectedDateStr string
	
	picker := NewCalendarPicker(func(t time.Time) {
		selectedDateStr = t.Format("2006-01-02")
	})

	// If current entry has a valid date, set it in the picker
	currentText := de.Entry.Text
	if currentText != "" {
		result := ParseGenealogyDate(currentText)
		if result.Valid && result.Date.Year() > 1 {
			picker.SetDate(result.Date)
			selectedDateStr = result.Date.Format("2006-01-02")
		}
	}

	// Create dialog content with instructions
	instructions := widget.NewLabel("Select a date or type below:")
	instructions.Wrapping = fyne.TextWrapWord

	helpText := widget.NewLabel("Tip: You can also type dates like:\n" +
		"• YYYY-MM-DD (e.g., 1945-05-08)\n" +
		"• DD Mon YYYY (e.g., 8 May 1945)\n" +
		"• Mon YYYY (e.g., May 1945)\n" +
		"• YYYY (e.g., 1945)\n" +
		"• With prefixes: abt 1945, bef 1945, aft 1945")
	helpText.Wrapping = fyne.TextWrapWord
	helpText.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		instructions,
		picker,
		widget.NewSeparator(),
		helpText,
	)

	// Create dialog
	d := dialog.NewCustom("Select Date", "OK", content, de.window)
	d.SetOnClosed(func() {
		if selectedDateStr != "" {
			de.Entry.SetText(selectedDateStr)
		}
	})
	
	d.Resize(fyne.NewSize(400, 500))
	d.Show()
}

// GetText returns the current text in the entry
func (de *DateEntry) GetText() string {
	return de.Entry.Text
}

// SetText sets the text in the entry
func (de *DateEntry) SetText(text string) {
	de.Entry.SetText(text)
	de.updateValidation(text)
}

// GetWidget returns the container widget for embedding in forms
func (de *DateEntry) GetWidget() *fyne.Container {
	return de.Container
}

// SetReadOnly sets the read-only state of the entry
func (de *DateEntry) SetReadOnly(readOnly bool) {
	de.Entry.Disable()
	de.CalendarButton.Disable()
}
