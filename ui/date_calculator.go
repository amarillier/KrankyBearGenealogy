package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showDateCalculatorDialog shows the date calculator dialog
func showDateCalculatorDialog(w fyne.Window) {
	// Create a new window for the calculator
	calcWin := fyne.CurrentApp().NewWindow("Date Calculator")
	calcWin.Resize(fyne.NewSize(1000, 1000))

	// Tab container for different calculation modes
	ageTab := createAgeCalculatorTab(calcWin)  // Start date → End date (age calculation)
	yearsTab := createYearsBetweenTab(calcWin) // Difference between two dates
	addSubTab := createAddSubtractTab(calcWin) // Add/subtract time from a date
	rangeTab := createDateRangeTab(calcWin)    // Birth year + age → death year
	durationTab := createDurationTab(calcWin)  // Duration with weeks

	tabs := container.NewAppTabs(
		container.NewTabItem("Age Calculator", ageTab),
		container.NewTabItem("Years Between", yearsTab),
		container.NewTabItem("Duration", durationTab),
		container.NewTabItem("Add/Subtract", addSubTab),
		container.NewTabItem("Date Range", rangeTab),
	)

	calcWin.SetContent(tabs)
	calcWin.Show()
}

// createAgeCalculatorTab creates the age calculator (start date → end date)
func createAgeCalculatorTab(w fyne.Window) *container.Scroll {
	var startDate, endDate time.Time

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	// Start date calendar picker
	startPicker := NewCalendarPicker(func(date time.Time) {
		startDate = date
		// Auto-calculate if both dates are set
		if !endDate.IsZero() && !endDate.Before(startDate) {
			age := calculateAge(startDate, endDate)
			resultLabel.SetText(age)
		}
	})
	startPicker.SetDate(time.Now())

	// End date calendar picker
	endPicker := NewCalendarPicker(func(date time.Time) {
		endDate = date
		// Auto-calculate if both dates are set
		if !startDate.IsZero() && !endDate.Before(startDate) {
			age := calculateAge(startDate, endDate)
			resultLabel.SetText(age)
		}
	})
	endPicker.SetDate(time.Now())

	calculateBtn := widget.NewButton("Calculate Age", func() {
		startDate = startPicker.GetDate()
		endDate = endPicker.GetDate()

		if startDate.IsZero() || endDate.IsZero() {
			dialog.ShowInformation("Input Required", "Please select both start and end dates.", w)
			return
		}

		if endDate.Before(startDate) {
			dialog.ShowError(fmt.Errorf("End date must be after start date."), w)
			return
		}

		age := calculateAge(startDate, endDate)
		resultLabel.SetText(age)
	})

	clearBtn := widget.NewButton("Clear", func() {
		startPicker.SetDate(time.Now())
		endPicker.SetDate(time.Now())
		startDate = time.Time{}
		endDate = time.Time{}
		resultLabel.SetText("")
	})

	description := widget.NewLabel("Calculate age between two dates:\nSelect a start date (e.g., birth date) and an end date (e.g., current date or another event) using the calendars or by typing in the date fields.")
	description.Wrapping = fyne.TextWrapWord

	// Layout: Two calendars side by side
	calendarsContainer := container.NewHBox(
		container.NewVBox(
			widget.NewLabelWithStyle("Start Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			startPicker,
		),
		container.NewVBox(
			widget.NewLabelWithStyle("End Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			endPicker,
		),
	)

	form := container.NewVBox(
		description,
		widget.NewSeparator(),
		calendarsContainer,
		container.NewHBox(calculateBtn, clearBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Result:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resultLabel,
	)

	return container.NewVScroll(form)
}

// createYearsBetweenTab creates the years between dates calculator
func createYearsBetweenTab(w fyne.Window) *container.Scroll {
	var date1, date2 time.Time

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	// First date calendar picker
	date1Picker := NewCalendarPicker(func(date time.Time) {
		date1 = date
		// Auto-calculate if both dates are set
		if !date2.IsZero() {
			diff := calculateDifference(date1, date2)
			resultLabel.SetText(diff)
		}
	})
	date1Picker.SetDate(time.Now())

	// Second date calendar picker
	date2Picker := NewCalendarPicker(func(date time.Time) {
		date2 = date
		// Auto-calculate if both dates are set
		if !date1.IsZero() {
			diff := calculateDifference(date1, date2)
			resultLabel.SetText(diff)
		}
	})
	date2Picker.SetDate(time.Now())

	calculateBtn := widget.NewButton("Calculate Difference", func() {
		date1 = date1Picker.GetDate()
		date2 = date2Picker.GetDate()

		if date1.IsZero() || date2.IsZero() {
			dialog.ShowInformation("Input Required", "Please select both dates.", w)
			return
		}

		diff := calculateDifference(date1, date2)
		resultLabel.SetText(diff)
	})

	clearBtn := widget.NewButton("Clear", func() {
		date1Picker.SetDate(time.Now())
		date2Picker.SetDate(time.Now())
		date1 = time.Time{}
		date2 = time.Time{}
		resultLabel.SetText("")
	})

	description := widget.NewLabel("Calculate time between two dates:\nSelect two dates using the calendars or by typing in the date fields. Useful for finding how many years between events (e.g., birth and marriage, marriage and death).")
	description.Wrapping = fyne.TextWrapWord

	// Layout: Two calendars side by side
	calendarsContainer := container.NewHBox(
		container.NewVBox(
			widget.NewLabelWithStyle("First Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			date1Picker,
		),
		container.NewVBox(
			widget.NewLabelWithStyle("Second Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			date2Picker,
		),
	)

	form := container.NewVBox(
		description,
		widget.NewSeparator(),
		calendarsContainer,
		container.NewHBox(calculateBtn, clearBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Result:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resultLabel,
	)

	return container.NewVScroll(form)
}

// createDurationTab creates the duration calculator (years, months, weeks, days)
func createDurationTab(w fyne.Window) *container.Scroll {
	var startDate, endDate time.Time

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	// Start date calendar picker
	startPicker := NewCalendarPicker(func(date time.Time) {
		startDate = date
		// Auto-calculate if both dates are set
		if !endDate.IsZero() && !endDate.Before(startDate) {
			duration := calculateDuration(startDate, endDate)
			resultLabel.SetText(duration)
		}
	})
	startPicker.SetDate(time.Now())

	// End date calendar picker
	endPicker := NewCalendarPicker(func(date time.Time) {
		endDate = date
		// Auto-calculate if both dates are set
		if !startDate.IsZero() && !endDate.Before(startDate) {
			duration := calculateDuration(startDate, endDate)
			resultLabel.SetText(duration)
		}
	})
	endPicker.SetDate(time.Now())

	calculateBtn := widget.NewButton("Calculate Duration", func() {
		startDate = startPicker.GetDate()
		endDate = endPicker.GetDate()

		if startDate.IsZero() || endDate.IsZero() {
			dialog.ShowInformation("Input Required", "Please select both start and end dates.", w)
			return
		}

		if endDate.Before(startDate) {
			dialog.ShowError(fmt.Errorf("End date must be after start date."), w)
			return
		}

		duration := calculateDuration(startDate, endDate)
		resultLabel.SetText(duration)
	})

	clearBtn := widget.NewButton("Clear", func() {
		startPicker.SetDate(time.Now())
		endPicker.SetDate(time.Now())
		startDate = time.Time{}
		endDate = time.Time{}
		resultLabel.SetText("")
	})

	description := widget.NewLabel("Calculate duration between two dates:\nSelect a start date and end date using the calendars or by typing in the date fields. Shows duration in years, months, weeks, and days. Useful for 'days until' calculations or 'days since' calculations.")
	description.Wrapping = fyne.TextWrapWord

	// Layout: Two calendars side by side
	calendarsContainer := container.NewHBox(
		container.NewVBox(
			widget.NewLabelWithStyle("Start Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			startPicker,
		),
		container.NewVBox(
			widget.NewLabelWithStyle("End Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			endPicker,
		),
	)

	form := container.NewVBox(
		description,
		widget.NewSeparator(),
		calendarsContainer,
		container.NewHBox(calculateBtn, clearBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Result:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resultLabel,
	)

	return container.NewVScroll(form)
}

// createAddSubtractTab creates the add/subtract time calculator
func createAddSubtractTab(w fyne.Window) *container.Scroll {
	var startDate time.Time

	yearsEntry := widget.NewEntry()
	yearsEntry.SetPlaceHolder("0")

	monthsEntry := widget.NewEntry()
	monthsEntry.SetPlaceHolder("0")

	daysEntry := widget.NewEntry()
	daysEntry.SetPlaceHolder("0")

	operationSelect := widget.NewRadioGroup([]string{"Add", "Subtract"}, nil)
	operationSelect.SetSelected("Add")

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	// Start date calendar picker
	startPicker := NewCalendarPicker(func(date time.Time) {
		startDate = date
	})

	calculateBtn := widget.NewButton("Calculate", func() {
		startDate = startPicker.GetDate()
		if startDate.IsZero() {
			dialog.ShowInformation("Input Required", "Please select a start date.", w)
			return
		}

		years := parseInt(yearsEntry.Text)
		months := parseInt(monthsEntry.Text)
		days := parseInt(daysEntry.Text)

		if years == 0 && months == 0 && days == 0 {
			dialog.ShowInformation("Input Required", "Please enter at least one time period (years, months, or days).", w)
			return
		}

		var resultDate time.Time
		if operationSelect.Selected == "Add" {
			resultDate = startDate.AddDate(years, months, days)
		} else {
			resultDate = startDate.AddDate(-years, -months, -days)
		}

		resultLabel.SetText(fmt.Sprintf("Result Date: %s\n(%s)", formatDate(resultDate), formatDateLong(resultDate)))
	})

	clearBtn := widget.NewButton("Clear", func() {
		startPicker.SetDate(time.Now())
		startDate = time.Time{}
		yearsEntry.SetText("0")
		monthsEntry.SetText("0")
		daysEntry.SetText("0")
		resultLabel.SetText("")
	})

	description := widget.NewLabel("Add or subtract time from a date:\nSelect a start date using the calendar or by typing, then enter years, months, and/or days to add or subtract. Useful for calculating dates (e.g., 'What date is 100 years before today?' or 'If someone was born in 1800, when would they be 75?').")
	description.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(
		description,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Start Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		startPicker,
		widget.NewSeparator(),
		widget.NewLabel("Operation:"),
		operationSelect,
		widget.NewLabel("Years:"),
		yearsEntry,
		widget.NewLabel("Months:"),
		monthsEntry,
		widget.NewLabel("Days:"),
		daysEntry,
		container.NewHBox(calculateBtn, clearBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Result:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resultLabel,
	)

	return container.NewVScroll(form)
}

// createDateRangeTab creates the date range calculator
func createDateRangeTab(w fyne.Window) *container.Scroll {
	var birthDate time.Time

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	ageEntry := widget.NewEntry()
	ageEntry.SetPlaceHolder("Age in years")
	ageEntry.OnChanged = func(text string) {
		// Auto-calculate if birth date is set
		if !birthDate.IsZero() && text != "" {
			age := parseInt(text)
			if age >= 0 && age <= 150 {
				birthYear := birthDate.Year()
				deathYear := birthYear + age
				resultLabel.SetText(fmt.Sprintf("If born in %d and lived %d years:\nDeath Year: %d\n\nDate Range: %d - %d", birthYear, age, deathYear, birthYear, deathYear))
			}
		}
	}

	// Birth date calendar picker (we'll extract the year)
	birthPicker := NewCalendarPicker(func(date time.Time) {
		birthDate = date
		// Auto-calculate if age is entered
		if ageEntry.Text != "" {
			age := parseInt(ageEntry.Text)
			if age >= 0 && age <= 150 {
				birthYear := birthDate.Year()
				deathYear := birthYear + age
				resultLabel.SetText(fmt.Sprintf("If born in %d and lived %d years:\nDeath Year: %d\n\nDate Range: %d - %d", birthYear, age, deathYear, birthYear, deathYear))
			}
		}
	})
	birthPicker.SetDate(time.Now())

	calculateBtn := widget.NewButton("Calculate Range", func() {
		birthDate = birthPicker.GetDate()
		if birthDate.IsZero() {
			dialog.ShowInformation("Input Required", "Please select a birth date.", w)
			return
		}

		ageStr := ageEntry.Text
		if ageStr == "" {
			dialog.ShowInformation("Input Required", "Please enter the age at death.", w)
			return
		}

		birthYear := birthDate.Year()
		if birthYear < 1 || birthYear > 9999 {
			dialog.ShowError(fmt.Errorf("Invalid birth year. Please select a date with a year between 1 and 9999."), w)
			return
		}

		age := parseInt(ageStr)
		if age < 0 || age > 150 {
			dialog.ShowError(fmt.Errorf("Invalid age. Please enter an age between 0 and 150."), w)
			return
		}

		deathYear := birthYear + age
		resultLabel.SetText(fmt.Sprintf("If born in %d and lived %d years:\nDeath Year: %d\n\nDate Range: %d - %d", birthYear, age, deathYear, birthYear, deathYear))
	})

	clearBtn := widget.NewButton("Clear", func() {
		birthPicker.SetDate(time.Now())
		birthDate = time.Time{}
		ageEntry.SetText("")
		resultLabel.SetText("")
	})

	description := widget.NewLabel("Calculate death year from birth date and age:\nSelect a birth date using the calendar (or type it), then enter the age at death. The year from the selected date will be used. Useful for estimating death dates when you only know birth year and approximate age at death.")
	description.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(
		description,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Birth Date:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("(Year from selected date will be used)"),
		birthPicker,
		widget.NewSeparator(),
		widget.NewLabel("Age at Death:"),
		ageEntry,
		container.NewHBox(calculateBtn, clearBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Result:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resultLabel,
	)

	return container.NewVScroll(form)
}

// parseInt parses an integer string, returns 0 if empty or invalid
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// calculateAge calculates age between two dates
func calculateAge(birth, death time.Time) string {
	years := death.Year() - birth.Year()
	months := int(death.Month()) - int(birth.Month())
	days := death.Day() - birth.Day()

	// Adjust if days are negative
	if days < 0 {
		months--
		// Get days in the previous month
		lastMonth := death.AddDate(0, -1, 0)
		days += daysInMonth(lastMonth.Year(), int(lastMonth.Month()))
	}

	// Adjust if months are negative
	if months < 0 {
		years--
		months += 12
	}

	// Calculate total days for more precision
	totalDays := int(death.Sub(birth).Hours() / 24)

	result := fmt.Sprintf("Age: %d years, %d months, %d days\n\n", years, months, days)
	result += fmt.Sprintf("Total: %d years\n", years)
	result += fmt.Sprintf("Total: %d days\n", totalDays)
	result += fmt.Sprintf("Total: %.1f years (decimal)", float64(totalDays)/365.25)

	return result
}

// calculateDifference calculates the difference between two dates
func calculateDifference(date1, date2 time.Time) string {
	var start, end time.Time
	if date1.Before(date2) {
		start, end = date1, date2
	} else {
		start, end = date2, date1
	}

	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	days := end.Day() - start.Day()

	// Adjust if days are negative
	if days < 0 {
		months--
		lastMonth := end.AddDate(0, -1, 0)
		days += daysInMonth(lastMonth.Year(), int(lastMonth.Month()))
	}

	// Adjust if months are negative
	if months < 0 {
		years--
		months += 12
	}

	totalDays := int(end.Sub(start).Hours() / 24)

	result := fmt.Sprintf("Difference: %d years, %d months, %d days\n\n", years, months, days)
	result += fmt.Sprintf("Total: %d years\n", years)
	result += fmt.Sprintf("Total: %d days\n", totalDays)
	result += fmt.Sprintf("Total: %.1f years (decimal)", float64(totalDays)/365.25)

	return result
}

// calculateDuration calculates duration between two dates with weeks
func calculateDuration(start, end time.Time) string {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	days := end.Day() - start.Day()

	// Adjust if days are negative
	if days < 0 {
		months--
		lastMonth := end.AddDate(0, -1, 0)
		days += daysInMonth(lastMonth.Year(), int(lastMonth.Month()))
	}

	// Adjust if months are negative
	if months < 0 {
		years--
		months += 12
	}

	// Calculate total days and weeks
	totalDays := int(end.Sub(start).Hours() / 24)
	totalWeeks := totalDays / 7
	remainingDaysAfterWeeks := totalDays % 7

	result := fmt.Sprintf("Duration: %d years, %d months, %d weeks, %d days\n\n", years, months, totalWeeks, remainingDaysAfterWeeks)
	result += fmt.Sprintf("Breakdown:\n")
	result += fmt.Sprintf("  Years: %d\n", years)
	result += fmt.Sprintf("  Months: %d\n", months)
	result += fmt.Sprintf("  Weeks: %d\n", totalWeeks)
	result += fmt.Sprintf("  Days: %d\n\n", remainingDaysAfterWeeks)
	result += fmt.Sprintf("Total: %d days\n", totalDays)
	result += fmt.Sprintf("Total: %d weeks and %d days\n", totalWeeks, remainingDaysAfterWeeks)
	result += fmt.Sprintf("Total: %.1f years (decimal)", float64(totalDays)/365.25)

	return result
}

// daysInMonth returns the number of days in a month
func daysInMonth(year, month int) int {
	// Create a date for the first day of the month
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	// Get the first day of next month and subtract one day
	lastDay := firstDay.AddDate(0, 1, 0).AddDate(0, 0, -1)
	return lastDay.Day()
}

// formatDate formats a date as YYYY-MM-DD
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// formatDateLong formats a date with full month name
func formatDateLong(t time.Time) string {
	return t.Format("January 2, 2006")
}
