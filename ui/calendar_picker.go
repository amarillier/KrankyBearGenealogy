package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CalendarPicker is a custom calendar date picker widget
type CalendarPicker struct {
	widget.BaseWidget
	selectedDate   time.Time
	displayMonth   time.Time
	onDateSelected  func(time.Time)
	entry          *widget.Entry
	monthLabel     *widget.Label
	prevMonthBtn   *widget.Button
	nextMonthBtn   *widget.Button
	prevYearBtn    *widget.Button
	nextYearBtn    *widget.Button
	calendarGrid   *fyne.Container
}

// NewCalendarPicker creates a new calendar picker widget
func NewCalendarPicker(onDateSelected func(time.Time)) *CalendarPicker {
	cp := &CalendarPicker{
		selectedDate:  time.Now(),
		displayMonth:  time.Now(),
		onDateSelected: onDateSelected,
		entry:         widget.NewEntry(),
	}
	cp.entry.SetPlaceHolder("YYYY-MM-DD or click calendar")
	cp.entry.OnChanged = func(text string) {
		if text != "" {
			if date, err := parseDate(text); err == nil {
				cp.selectedDate = date
				cp.displayMonth = date
				cp.Refresh()
				if cp.onDateSelected != nil {
					cp.onDateSelected(date)
				}
			}
		}
	}
	
	cp.monthLabel = widget.NewLabel("")
	cp.monthLabel.Alignment = fyne.TextAlignCenter
	
	cp.prevMonthBtn = widget.NewButton("◀", func() {
		cp.displayMonth = cp.displayMonth.AddDate(0, -1, 0)
		cp.Refresh()
	})
	
	cp.nextMonthBtn = widget.NewButton("▶", func() {
		cp.displayMonth = cp.displayMonth.AddDate(0, 1, 0)
		cp.Refresh()
	})
	
	cp.prevYearBtn = widget.NewButton("◀◀", func() {
		cp.displayMonth = cp.displayMonth.AddDate(-1, 0, 0)
		cp.Refresh()
	})
	
	cp.nextYearBtn = widget.NewButton("▶▶", func() {
		cp.displayMonth = cp.displayMonth.AddDate(1, 0, 0)
		cp.Refresh()
	})
	
	cp.calendarGrid = container.NewGridWithColumns(7)
	
	cp.ExtendBaseWidget(cp)
	cp.updateMonthLabel()
	cp.updateCalendarGrid()
	return cp
}

// SetDate sets the selected date
func (cp *CalendarPicker) SetDate(date time.Time) {
	cp.selectedDate = date
	cp.displayMonth = date
	cp.entry.SetText(date.Format("2006-01-02"))
	cp.updateMonthLabel()
	cp.updateCalendarGrid()
}

// GetDate returns the selected date
func (cp *CalendarPicker) GetDate() time.Time {
	return cp.selectedDate
}

// CreateRenderer creates the renderer for the calendar picker
func (cp *CalendarPicker) CreateRenderer() fyne.WidgetRenderer {
	// Entry at top
	entryContainer := container.NewBorder(nil, nil, nil, nil, cp.entry)
	
	// Controls: prev month, prev year, month label, next year, next month
	controls := container.NewBorder(nil, nil,
		cp.prevMonthBtn,
		cp.nextMonthBtn,
		container.NewHBox(cp.prevYearBtn, cp.monthLabel, cp.nextYearBtn))
	
	// Day headers
	dayHeaders := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	headerContainer := container.NewGridWithColumns(7)
	for _, day := range dayHeaders {
		label := widget.NewLabel(day)
		label.Alignment = fyne.TextAlignCenter
		headerContainer.Add(label)
	}
	
	// Main content: entry, controls, headers, calendar grid
	content := container.NewVBox(
		entryContainer,
		controls,
		headerContainer,
		cp.calendarGrid,
	)
	
	return &calendarPickerRenderer{
		cp:      cp,
		content: content,
	}
}

type calendarPickerRenderer struct {
	cp      *CalendarPicker
	content *fyne.Container
}

func (r *calendarPickerRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *calendarPickerRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *calendarPickerRenderer) Refresh() {
	r.cp.updateMonthLabel()
	r.cp.updateCalendarGrid()
}

func (r *calendarPickerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *calendarPickerRenderer) Destroy() {}

func (cp *CalendarPicker) updateMonthLabel() {
	cp.monthLabel.SetText(cp.displayMonth.Format("January 2006"))
}

func (cp *CalendarPicker) updateCalendarGrid() {
	// Clear existing grid
	cp.calendarGrid.Objects = nil

	// Get first day of month
	firstOfMonth := time.Date(cp.displayMonth.Year(), cp.displayMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstDayOfWeek := int(firstOfMonth.Weekday())
	
	// Get last day of month
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	daysInMonth := lastOfMonth.Day()

	// Fill in days before month starts
	for i := 0; i < firstDayOfWeek; i++ {
		blank := widget.NewLabel("")
		cp.calendarGrid.Add(blank)
	}

	// Fill in days of month
	today := time.Now()
	for dayNum := 1; dayNum <= daysInMonth; dayNum++ {
		currentDate := time.Date(cp.displayMonth.Year(), cp.displayMonth.Month(), dayNum, 0, 0, 0, 0, time.UTC)
		
		dayBtn := widget.NewButton(fmt.Sprintf("%d", dayNum), func(date time.Time) func() {
			return func() {
				cp.selectedDate = date
				cp.entry.SetText(date.Format("2006-01-02"))
				cp.Refresh()
				if cp.onDateSelected != nil {
					cp.onDateSelected(date)
				}
			}
		}(currentDate))

		// Highlight selected date
		if currentDate.Year() == cp.selectedDate.Year() &&
			currentDate.Month() == cp.selectedDate.Month() &&
			currentDate.Day() == cp.selectedDate.Day() {
			dayBtn.Importance = widget.HighImportance
		}

		// Highlight today
		if currentDate.Year() == today.Year() &&
			currentDate.Month() == today.Month() &&
			currentDate.Day() == today.Day() {
			dayBtn.Importance = widget.MediumImportance
		}

		cp.calendarGrid.Add(dayBtn)
	}

	// Fill in remaining days to complete grid (up to 42 cells total for 6 weeks)
	totalCells := firstDayOfWeek + daysInMonth
	for totalCells < 42 {
		blank := widget.NewLabel("")
		cp.calendarGrid.Add(blank)
		totalCells++
	}

	cp.calendarGrid.Refresh()
}
