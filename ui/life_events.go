package ui

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// commonLifeEventTypes are suggested (not enforced) values for the event
// type field. Deliberately excludes anything like "Residence"/"Moved to" -
// tracking a sequence of homes over time was intentionally left out.
var commonLifeEventTypes = []string{
	"Immigration",
	"Naturalization",
	"Occupation",
	"Military Service",
	"Education",
	"Religious Event",
	"Other",
}

// showLifeEventsManager shows the life events manager window for a person.
func showLifeEventsManager(parentWindow fyne.Window, s *store.Store, personID int64, personName string) {
	w := fyne.CurrentApp().NewWindow(fmt.Sprintf("Life Events: %s", personName))

	var rebuildContent func()
	rebuildContent = func() {
		events, err := s.GetEvents(personID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load life events: %w", err), parentWindow)
			return
		}

		sort.Slice(events, func(i, j int) bool {
			return parseDateForSort(events[i].Date).Before(parseDateForSort(events[j].Date))
		})

		content := container.NewVBox()

		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Life Events for %s (%d entries)", personName, len(events)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		addBtn := widget.NewButton("+ Add Life Event", func() {
			showAddLifeEventDialog(w, s, personID, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(events) == 0 {
			content.Add(widget.NewLabel("No life events recorded yet."))
			content.Add(widget.NewLabel("Track one-off happenings like immigration, occupation, or military service."))
		} else {
			for _, event := range events {
				eventCopy := event
				content.Add(makeLifeEventCard(eventCopy, s, rebuildContent, w))
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(700, 700))
		w.SetContent(scroll)
	}

	rebuildContent()
	w.Resize(fyne.NewSize(800, 800))
	w.Show()
}

// makeLifeEventCard creates a UI card for a single life event.
func makeLifeEventCard(event store.Event, s *store.Store, onUpdate func(), w fyne.Window) *fyne.Container {
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("📅 %s", event.Type),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	dateText := event.Date
	if event.DateEnd != "" {
		dateText = fmt.Sprintf("%s – %s", event.Date, event.DateEnd)
	}
	dateLabel := widget.NewLabel(fmt.Sprintf("Date: %s", dateText))

	placeLabel := widget.NewLabel("")
	if event.Place != "" {
		placeLabel.SetText(fmt.Sprintf("Place: %s", event.Place))
	}

	notesLabel := widget.NewLabel("")
	if event.Note != "" {
		notesLabel.SetText(fmt.Sprintf("Notes: %s", event.Note))
		notesLabel.Wrapping = fyne.TextWrapWord
	}

	editBtn := widget.NewButton("Edit", func() {
		showEditLifeEventDialog(w, s, &event, onUpdate)
	})

	deleteBtn := widget.NewButton("Delete", func() {
		dialog.ShowConfirm("Delete Life Event",
			fmt.Sprintf("Are you sure you want to delete this \"%s\" event?", event.Type),
			func(confirmed bool) {
				if confirmed {
					if err := s.DeleteEvent(event.ID); err != nil {
						dialog.ShowError(err, w)
					} else {
						onUpdate()
					}
				}
			}, w)
	})

	buttons := container.NewHBox(editBtn, deleteBtn)

	return container.NewVBox(
		header,
		dateLabel,
		placeLabel,
		notesLabel,
		buttons,
	)
}

// showAddLifeEventDialog shows the dialog to add a new life event.
func showAddLifeEventDialog(w fyne.Window, s *store.Store, personID int64, onSave func()) {
	typeEntry := widget.NewSelectEntry(commonLifeEventTypes)
	typeEntry.SetPlaceHolder("e.g. Immigration, Occupation, Military Service...")

	dateEntry := NewDateEntry(w)

	dateEndEntry := NewDateEntry(w)
	dateEndEntry.Entry.SetPlaceHolder("Optional - for events with a duration, e.g. military service")

	placeAutocomplete := NewPlaceAutocompleteContainer(func() *store.Store { return s }, "")

	noteEntry := widget.NewMultiLineEntry()
	noteEntry.SetPlaceHolder("Additional details (optional)")
	noteEntry.SetMinRowsVisible(3)

	var d dialog.Dialog

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Type", Widget: typeEntry},
			{Text: "Date", Widget: dateEntry.GetWidget()},
			{Text: "End Date", Widget: dateEndEntry.GetWidget()},
			{Text: "Place", Widget: placeAutocomplete.Container},
			{Text: "Notes", Widget: noteEntry},
		},
		OnSubmit: func() {
			eventType := strings.TrimSpace(typeEntry.Text)
			if eventType == "" {
				dialog.ShowError(fmt.Errorf("Event type is required"), w)
				return
			}

			event := &store.Event{
				PersonID: personID,
				Type:     eventType,
				Date:     strings.TrimSpace(dateEntry.GetText()),
				DateEnd:  strings.TrimSpace(dateEndEntry.GetText()),
				Place:    strings.TrimSpace(placeAutocomplete.GetText()),
				Note:     strings.TrimSpace(noteEntry.Text),
			}

			if err := s.CreateEvent(event); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to save life event: %w", err), w)
				return
			}

			d.Hide()
			onSave()
		},
		OnCancel: func() {
			d.Hide()
		},
	}

	// No separate dismiss button - the form already renders its own
	// Submit/Cancel, and both now close the dialog themselves. A third
	// "Close" button previously left the dialog open after Submit with no
	// visible feedback, inviting repeated clicks and duplicate records.
	d = dialog.NewCustomWithoutButtons("Add Life Event", form, w)
	d.Resize(fyne.NewSize(500, 460))
	d.Show()
}

// showEditLifeEventDialog shows the dialog to edit an existing life event.
func showEditLifeEventDialog(w fyne.Window, s *store.Store, event *store.Event, onSave func()) {
	typeEntry := widget.NewSelectEntry(commonLifeEventTypes)
	typeEntry.SetText(event.Type)

	dateEntry := NewDateEntry(w)
	dateEntry.SetText(event.Date)

	dateEndEntry := NewDateEntry(w)
	dateEndEntry.Entry.SetPlaceHolder("Optional - for events with a duration, e.g. military service")
	dateEndEntry.SetText(event.DateEnd)

	placeAutocomplete := NewPlaceAutocompleteContainer(func() *store.Store { return s }, event.Place)

	noteEntry := widget.NewMultiLineEntry()
	noteEntry.SetText(event.Note)
	noteEntry.SetMinRowsVisible(3)

	var d dialog.Dialog

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Type", Widget: typeEntry},
			{Text: "Date", Widget: dateEntry.GetWidget()},
			{Text: "End Date", Widget: dateEndEntry.GetWidget()},
			{Text: "Place", Widget: placeAutocomplete.Container},
			{Text: "Notes", Widget: noteEntry},
		},
		OnSubmit: func() {
			eventType := strings.TrimSpace(typeEntry.Text)
			if eventType == "" {
				dialog.ShowError(fmt.Errorf("Event type is required"), w)
				return
			}

			event.Type = eventType
			event.Date = strings.TrimSpace(dateEntry.GetText())
			event.DateEnd = strings.TrimSpace(dateEndEntry.GetText())
			event.Place = strings.TrimSpace(placeAutocomplete.GetText())
			event.Note = strings.TrimSpace(noteEntry.Text)

			if err := s.UpdateEvent(event); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to update life event: %w", err), w)
				return
			}

			d.Hide()
			onSave()
		},
		OnCancel: func() {
			d.Hide()
		},
	}

	d = dialog.NewCustomWithoutButtons("Edit Life Event", form, w)
	d.Resize(fyne.NewSize(500, 460))
	d.Show()
}
