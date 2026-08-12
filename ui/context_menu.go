package ui

import (
	"fmt"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	
	"genealogy/store"
)

// TappableContainer is a custom widget that supports both primary (left) and secondary (right) taps
type TappableContainer struct {
	widget.BaseWidget
	content          fyne.CanvasObject
	onTapped         func()
	onTappedSecondary func(*fyne.PointEvent)
}

// NewTappableContainer creates a new tappable container
func NewTappableContainer(content fyne.CanvasObject, onTapped func(), onTappedSecondary func(*fyne.PointEvent)) *TappableContainer {
	tc := &TappableContainer{
		content:          content,
		onTapped:         onTapped,
		onTappedSecondary: onTappedSecondary,
	}
	tc.ExtendBaseWidget(tc)
	return tc
}

// CreateRenderer implements fyne.Widget
func (tc *TappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(tc.content)
}

// Tapped handles primary (left) click
func (tc *TappableContainer) Tapped(e *fyne.PointEvent) {
	if tc.onTapped != nil {
		tc.onTapped()
	}
}

// TappedSecondary handles secondary (right) click
func (tc *TappableContainer) TappedSecondary(e *fyne.PointEvent) {
	if tc.onTappedSecondary != nil {
		tc.onTappedSecondary(e)
	}
}

// PersonContextMenu creates and shows a context menu for a person
func ShowPersonContextMenu(
	person *store.Person,
	s *store.Store,
	w fyne.Window,
	pos fyne.Position,
	onNavigate func(int64),
	refreshView func(),
) {
	if person == nil {
		return
	}
	
	// Build menu items
	var items []*fyne.MenuItem
	
	// === NAVIGATION ===
	items = append(items, fyne.NewMenuItem("Set as Focus Person", func() {
		if onNavigate != nil {
			onNavigate(person.ID)
		}
	}))
	
	// View in different views
	items = append(items, fyne.NewMenuItem("View in Fan Chart...", func() {
		showFanChartDialog(w, s, person.ID, onNavigate)
	}))

	items = append(items, fyne.NewMenuItem("View in Descendant Chart...", func() {
		showDescendantChartDialog(w, s, person.ID, onNavigate)
	}))

	items = append(items, fyne.NewMenuItem("View on Map...", func() {
		showMapViewDialog(w, s, person.ID, onNavigate)
	}))
	
	// View Media (if person has media)
	if media, err := s.GetMediaForPerson(person.ID); err == nil && len(media) > 0 {
		items = append(items, fyne.NewMenuItem(fmt.Sprintf("View Media (%d)...", len(media)), func() {
			showMediaManager(w, s, person.ID, formatPersonName(*person))
		}))
	}
	
	items = append(items, fyne.NewMenuItemSeparator())
	
	// === QUICK ACTIONS ===
	
	// Bookmark toggle
	if person.Bookmarked {
		items = append(items, fyne.NewMenuItem("Remove Bookmark", func() {
			person.Bookmarked = false
			if err := s.UpdatePerson(person); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to update bookmark: %v", err), w)
			} else {
				if refreshView != nil {
					refreshView()
				}
			}
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Add Bookmark", func() {
			person.Bookmarked = true
			if err := s.UpdatePerson(person); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to update bookmark: %v", err), w)
			} else {
				if refreshView != nil {
					refreshView()
				}
			}
		}))
	}
	
	// To-Do
	items = append(items, fyne.NewMenuItem("Add To-Do...", func() {
		// Open the To-Do dialog for this person
		showAddTodoDialogForPerson(s, w, person.ID, refreshView)
	}))
	
	// View To-Dos
	if count, _ := s.CountPendingTodosForPerson(person.ID); count > 0 {
		items = append(items, fyne.NewMenuItem(fmt.Sprintf("View To-Dos (%d)", count), func() {
			showTodosForPerson(s, w, person.ID, refreshView)
		}))
	}
	
	items = append(items, fyne.NewMenuItemSeparator())
	
	// === SOURCES & RESEARCH ===
	
	// Sources
	if count, _ := s.CountCitationsForPerson(person.ID); count > 0 {
		items = append(items, fyne.NewMenuItem(fmt.Sprintf("View Sources (%d)", count), func() {
			// For now, open the main Sources Library (can be filtered later)
			showSourcesLibrary(w, s)
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Add Source...", func() {
			// For now, open the main Sources Library
			showSourcesLibrary(w, s)
		}))
	}
	
	// Research Log
	if count, _ := s.CountResearchLogsForPerson(person.ID); count > 0 {
		items = append(items, fyne.NewMenuItem(fmt.Sprintf("View Research Log (%d)", count), func() {
			// For now, open the main Research Log Manager (can be filtered later)
			showResearchLogManager(w, s)
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Add Research Log...", func() {
			// For now, open the main Research Log Manager
			showResearchLogManager(w, s)
		}))
	}
	
	items = append(items, fyne.NewMenuItemSeparator())
	
	// === COPY INFORMATION ===
	
	items = append(items, fyne.NewMenuItem("Copy Name", func() {
		w.Clipboard().SetContent(formatPersonName(*person))
	}))
	
	items = append(items, fyne.NewMenuItem("Copy ID", func() {
		w.Clipboard().SetContent(fmt.Sprintf("%d", person.ID))
	}))
	
	items = append(items, fyne.NewMenuItemSeparator())
	
	// === REPORTS ===
	
	items = append(items, fyne.NewMenuItem("Generate Ancestor Report...", func() {
		showAncestorReport(w, s, person.ID, onNavigate)
	}))
	
	items = append(items, fyne.NewMenuItem("Generate Descendant Report...", func() {
		showDescendantReport(w, s, person.ID, onNavigate)
	}))
	
	items = append(items, fyne.NewMenuItem("Family Group Sheet...", func() {
		showFamilyGroupSheet(w, s, person.ID, onNavigate)
	}))
	
	// Create menu
	menu := fyne.NewMenu("", items...)
	
	// Show popup menu at position
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// Helper function to show add todo dialog for a specific person
func showAddTodoDialogForPerson(s *store.Store, w fyne.Window, personID int64, refreshView func()) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %v", err), w)
		return
	}
	
	// Create entry for todo description
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Enter to-do description...")
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.SetMinRowsVisible(3)
	
	// Priority selector
	prioritySelect := widget.NewSelect([]string{"Low", "Medium", "High"}, nil)
	prioritySelect.SetSelected("Medium")
	
	formItems := []*widget.FormItem{
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Priority", prioritySelect),
	}
	
	dialog.ShowForm("Add To-Do for "+formatPersonName(*person), "Add", "Cancel", formItems, func(submitted bool) {
		if submitted && descEntry.Text != "" {
			todo := &store.ResearchTodo{
				PersonID:    personID,
				Description: descEntry.Text,
				Priority:    prioritySelect.Selected,
				Status:      "pending",
			}
			
			if err := s.CreateTodo(todo); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to create to-do: %v", err), w)
			} else {
				if refreshView != nil {
					refreshView()
				}
			}
		}
	}, w)
}

// Helper function to show todos for a person
func showTodosForPerson(s *store.Store, w fyne.Window, personID int64, refreshView func()) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %v", err), w)
		return
	}
	
	todos, err := s.GetTodosForPerson(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load to-dos: %v", err), w)
		return
	}
	
	if len(todos) == 0 {
		dialog.ShowInformation("To-Dos", "No to-dos found for "+formatPersonName(*person), w)
		return
	}
	
	// Build list of todos
	var todoList []string
	for _, todo := range todos {
		status := "☐"
		if todo.Status == "completed" {
			status = "✓"
		}
		todoList = append(todoList, fmt.Sprintf("%s [%s] %s", status, todo.Priority, todo.Description))
	}
	
	list := widget.NewList(
		func() int { return len(todoList) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(todoList[i])
		},
	)
	
	content := container.NewBorder(
		widget.NewLabel(fmt.Sprintf("To-Dos for %s:", formatPersonName(*person))),
		nil, nil, nil,
		list,
	)
	content.Resize(fyne.NewSize(500, 400))
	
	dialog.ShowCustom("To-Dos", "Close", content, w)
}

// Note: showAncestorReport and showDescendantReport are already defined in app.go
