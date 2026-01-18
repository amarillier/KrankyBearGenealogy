package ui

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// IndividualView displays a sortable table of all people.
type IndividualView struct {
	widget.BaseWidget
	store          *store.Store
	window         fyne.Window
	onNavigate     func(personID int64)
	onEdit         func(personID int64)
	onSwitchFamily func(personID int64)
	onSwitchPedigree func(personID int64)
	content        *fyne.Container
	people         []store.Person
	filteredPeople []store.Person
	selectedIndex  int
	sortColumn     int
	sortAscending  bool
}

// NewIndividualView creates a new individual list widget.
func NewIndividualView(s *store.Store, w fyne.Window, onNavigate func(personID int64), onEdit func(personID int64)) *IndividualView {
	iv := &IndividualView{
		store:         s,
		window:        w,
		onNavigate:    onNavigate,
		onEdit:        onEdit,
		selectedIndex: -1,
		sortColumn:    1, // Default sort by name
		sortAscending: true,
	}
	iv.ExtendBaseWidget(iv)
	return iv
}

// SetSwitchHandlers sets the callbacks for switching views.
func (iv *IndividualView) SetSwitchHandlers(onSwitchFamily, onSwitchPedigree func(personID int64)) {
	iv.onSwitchFamily = onSwitchFamily
	iv.onSwitchPedigree = onSwitchPedigree
}

// Refresh reloads the people list.
func (iv *IndividualView) Refresh() {
	people, err := iv.store.GetPeople()
	if err != nil {
		return
	}
	iv.people = people
	iv.filteredPeople = make([]store.Person, len(people))
	copy(iv.filteredPeople, people)
	iv.sortPeople()
	iv.refresh()
}

// SetPerson highlights a specific person in the table and scrolls to them.
func (iv *IndividualView) SetPerson(p *store.Person) {
	if p == nil {
		return
	}
	// Find the person in the FULL people list (not filtered)
	for i, person := range iv.people {
		if person.ID == p.ID {
			iv.selectedIndex = i
			iv.refresh()
			iv.Refresh() // Force widget refresh
			return
		}
	}
	// Not found - just refresh without selection
	iv.selectedIndex = -1
	iv.refresh()
	iv.Refresh()
}

// SetFilteredPeople is no longer used - Individual view always shows all people.
// This is kept for compatibility but doesn't change the display.
func (iv *IndividualView) SetFilteredPeople(people []store.Person) {
	// Individual view always shows all people, just updates selection
	// Do nothing here - selection is handled by SetPerson
}

// CreateRenderer implements the widget interface.
func (iv *IndividualView) CreateRenderer() fyne.WidgetRenderer {
	iv.content = container.NewMax()
	iv.Refresh()
	return widget.NewSimpleRenderer(iv.content)
}

func (iv *IndividualView) refresh() {
	if iv.content == nil {
		return
	}
	iv.content.Objects = nil

	// Action buttons (no search - use left panel search instead)
	familyBtn := widget.NewButton("Go to Family View", func() {
		if iv.selectedIndex >= 0 && iv.selectedIndex < len(iv.people) {
			if iv.onSwitchFamily != nil {
				iv.onSwitchFamily(iv.people[iv.selectedIndex].ID)
			}
		}
	})

	pedigreeBtn := widget.NewButton("Go to Pedigree View", func() {
		if iv.selectedIndex >= 0 && iv.selectedIndex < len(iv.people) {
			if iv.onSwitchPedigree != nil {
				iv.onSwitchPedigree(iv.people[iv.selectedIndex].ID)
			}
		}
	})

	editBtn := widget.NewButton("Edit Person", func() {
		if iv.selectedIndex >= 0 && iv.selectedIndex < len(iv.people) {
			if iv.onEdit != nil {
				iv.onEdit(iv.people[iv.selectedIndex].ID)
			}
		}
	})

	infoLabel := widget.NewLabel(fmt.Sprintf("Showing all %d people (use left panel to search)", len(iv.people)))

	toolbar := container.NewHBox(familyBtn, pedigreeBtn, editBtn, layout.NewSpacer(), infoLabel)

	// Create table - this returns a container with header and scrollable list
	table := iv.createTable()

	// Use border layout to give table maximum vertical space
	mainLayout := container.NewBorder(
		toolbar,  // top
		nil,      // bottom
		nil,      // left
		nil,      // right
		table,    // center - this will expand to fill available space
	)
	
	iv.content.Objects = []fyne.CanvasObject{mainLayout}
	iv.content.Refresh()
}

// createTable creates the sortable table widget.
func (iv *IndividualView) createTable() fyne.CanvasObject {
	// Column headers
	headers := []string{"#", "Full Name", "Sex", "Birth Date", "Birth Place", "Death Date", "Death Place"}
	
	headerRow := container.NewHBox()
	for i, h := range headers {
		col := i
		btn := widget.NewButton(h, func() {
			iv.sortByColumn(col)
		})
		if col == iv.sortColumn {
			if iv.sortAscending {
				btn.SetText(h + " ▲")
			} else {
				btn.SetText(h + " ▼")
			}
		}
		headerRow.Add(btn)
	}

	// Data rows - show ALL people (not filtered), don't truncate
	list := widget.NewList(
		func() int {
			return len(iv.people)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("genealogy")
		},
		func(i int, o fyne.CanvasObject) {
			if i >= len(iv.people) {
				return
			}
			p := iv.people[i]
			label := o.(*widget.Label)
			
			// Don't truncate - show full data with proper spacing
			row := fmt.Sprintf("%-6d  %-40s  %-6s  %-20s  %-40s  %-20s  %-40s",
				i+1,
				p.GivenName+" "+p.Surname,
				p.Gender,
				p.BirthDate,
				p.BirthPlace,
				p.DeathDate,
				p.DeathPlace,
			)
			label.SetText(row)
		},
	)

	list.OnSelected = func(i int) {
		iv.selectedIndex = i
		// Notify parent when selection changes in the table
		if i >= 0 && i < len(iv.people) {
			if iv.onNavigate != nil {
				iv.onNavigate(iv.people[i].ID)
			}
		}
	}

	// Select and scroll to the current selected index if set
	if iv.selectedIndex >= 0 && iv.selectedIndex < len(iv.people) {
		list.Select(iv.selectedIndex)
		// Scroll to a few rows before to ensure selected row is visible near top
		scrollIndex := iv.selectedIndex - 2
		if scrollIndex < 0 {
			scrollIndex = 0
		}
		list.ScrollTo(scrollIndex)
	}

	// List widget handles its own scrolling - just return it with header
	// Use NewMax to make the list fill all available space
	return container.NewBorder(headerRow, nil, nil, nil, container.NewMax(list))
}

// sortByColumn sorts the filtered people by the specified column.
func (iv *IndividualView) sortByColumn(col int) {
	if iv.sortColumn == col {
		iv.sortAscending = !iv.sortAscending
	} else {
		iv.sortColumn = col
		iv.sortAscending = true
	}
	iv.sortPeople()
	iv.refresh()
}

// sortPeople sorts the filtered people list.
func (iv *IndividualView) sortPeople() {
	sort.Slice(iv.filteredPeople, func(i, j int) bool {
		less := false
		switch iv.sortColumn {
		case 0: // Index
			less = i < j
		case 1: // Name
			nameI := iv.filteredPeople[i].Surname + " " + iv.filteredPeople[i].GivenName
			nameJ := iv.filteredPeople[j].Surname + " " + iv.filteredPeople[j].GivenName
			less = strings.ToLower(nameI) < strings.ToLower(nameJ)
		case 2: // Sex
			less = iv.filteredPeople[i].Gender < iv.filteredPeople[j].Gender
		case 3: // Birth Date
			less = iv.filteredPeople[i].BirthDate < iv.filteredPeople[j].BirthDate
		case 4: // Birth Place
			less = strings.ToLower(iv.filteredPeople[i].BirthPlace) < strings.ToLower(iv.filteredPeople[j].BirthPlace)
		case 5: // Death Date
			less = iv.filteredPeople[i].DeathDate < iv.filteredPeople[j].DeathDate
		case 6: // Death Place
			less = strings.ToLower(iv.filteredPeople[i].DeathPlace) < strings.ToLower(iv.filteredPeople[j].DeathPlace)
		}
		
		if !iv.sortAscending {
			return !less
		}
		return less
	})
}

// No longer needed - filtering happens via left panel

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
