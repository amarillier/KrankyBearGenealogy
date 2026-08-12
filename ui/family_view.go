package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// FamilyView displays a person with their parents, spouse, and children in a PAF-style layout.
type FamilyView struct {
	widget.BaseWidget
	store           *store.Store
	currentPerson   *store.Person
	currentSpouse   *store.Person     // Currently displayed spouse (for multi-marriage support)
	currentMarriage *store.SpouseInfo // Currently displayed marriage details
	window          fyne.Window
	onNavigate      func(personID int64)
	content         *fyne.Container
}

// NewFamilyView creates a new family view widget.
func NewFamilyView(s *store.Store, w fyne.Window, onNavigate func(personID int64)) *FamilyView {
	fv := &FamilyView{
		store:      s,
		window:     w,
		onNavigate: onNavigate,
	}
	fv.ExtendBaseWidget(fv)
	return fv
}

// SetPerson updates the view to show a specific person and their family.
func (fv *FamilyView) SetPerson(p *store.Person) {
	fv.currentPerson = p
	// Reset to first/primary spouse when person changes
	fv.currentSpouse = nil
	fv.currentMarriage = nil
	fv.refresh()
	fv.Refresh() // Force widget refresh
}

// CreateRenderer implements the widget interface.
func (fv *FamilyView) CreateRenderer() fyne.WidgetRenderer {
	fv.content = container.NewVBox()
	fv.refresh()
	return widget.NewSimpleRenderer(fv.content)
}

func (fv *FamilyView) refresh() {
	if fv.content == nil {
		return
	}
	fv.content.Objects = nil

	if fv.currentPerson == nil {
		fv.content.Add(widget.NewLabel("No person selected"))
		fv.content.Refresh()
		return
	}

	p := fv.currentPerson

	// Get all spouses
	allSpouses, _ := fv.store.GetSpouses(p.ID)

	// If we don't have a current spouse selected, select the most relevant one
	if fv.currentSpouse == nil && len(allSpouses) > 0 {
		// Separate current marriages (no end date/reason) from ended marriages
		var currentMarriages []store.SpouseInfo
		var endedMarriages []store.SpouseInfo

		for _, spouse := range allSpouses {
			if spouse.DivorceDate == "" && spouse.SeparationDate == "" && spouse.EndReason == "" {
				currentMarriages = append(currentMarriages, spouse)
			} else {
				endedMarriages = append(endedMarriages, spouse)
			}
		}

		// Default to most recent current marriage, or most recent ended marriage
		var selectedSpouses []store.SpouseInfo
		if len(currentMarriages) > 0 {
			selectedSpouses = currentMarriages
		} else {
			selectedSpouses = endedMarriages
		}

		// Sort by marriage date (most recent first)
		sort.Slice(selectedSpouses, func(i, j int) bool {
			date1 := parseDateForSort(selectedSpouses[i].MarriageDate)
			date2 := parseDateForSort(selectedSpouses[j].MarriageDate)
			if !date1.IsZero() && !date2.IsZero() {
				return date1.After(date2) // Most recent first
			}
			if !date1.IsZero() && date2.IsZero() {
				return true // Dated marriage comes before undated
			}
			if date1.IsZero() && !date2.IsZero() {
				return false // Undated marriage comes after dated
			}
			return false
		})

		fv.currentMarriage = &selectedSpouses[0]
		fv.currentSpouse = &selectedSpouses[0].Spouse
	}

	// --- PARENTS SECTION ---
	parentsBox := container.NewVBox(
		widget.NewLabelWithStyle("Parents", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	parents, _ := fv.store.GetRelatedPeople(p.ID, "parent")
	if len(parents) == 0 {
		parentsBox.Add(widget.NewLabel("No parents recorded"))
	} else {
		// Sort parents: father (M) first, then mother (F)
		sort.Slice(parents, func(i, j int) bool {
			if parents[i].Gender == "M" && parents[j].Gender != "M" {
				return true
			}
			if parents[i].Gender != "M" && parents[j].Gender == "M" {
				return false
			}
			return false
		})

		for _, parent := range parents {
			parentsBox.Add(fv.makePersonCard(parent, true))
		}
	}

	// --- CURRENT PERSON SECTION ---
	personBox := container.NewVBox(
		widget.NewLabelWithStyle("Individual", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		fv.makeCurrentPersonCard(*p),
	)

	// --- SPOUSE SECTION (SINGLE SPOUSE) ---
	spouseBox := container.NewVBox()

	// Row 1: Spouse label and marriage-related buttons
	spouseHeaderRow1 := container.NewHBox(
		widget.NewLabelWithStyle("Spouse", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	if len(allSpouses) > 1 {
		otherMarriagesBtn := widget.NewButton("Other Marriages...", func() {
			fv.showOtherMarriagesDialog(allSpouses)
		})
		spouseHeaderRow1.Add(otherMarriagesBtn)
	}

	if fv.currentSpouse != nil && fv.currentMarriage != nil {
		editMarriageBtn := widget.NewButton("Edit Marriage...", func() {
			fv.showEditMarriageDialog(fv.currentMarriage, fv.currentSpouse.ID)
		})
		spouseHeaderRow1.Add(editMarriageBtn)
	}

	// Row 2: Person research and management buttons
	spouseHeaderRow2 := container.NewHBox(
		// Add Manage To-Do button (always visible for current person)
		widget.NewButton("📝 Manage To-Do", func() {
			personName := formatPersonName(*p)
			showTodoManager(fv.window, fv.store, p.ID, personName)
		}),
		// Add Manage Sources button (always visible for current person)
		widget.NewButton("📚 Manage Sources", func() {
			personName := formatPersonName(*p)
			showCitationManager(fv.window, fv.store, p.ID, personName)
		}),
		// Add Research Log button (always visible for current person)
		widget.NewButton("🔍 Research Log", func() {
			personName := formatPersonName(*p)
			showResearchLogForPerson(fv.window, fv.store, p.ID, personName)
		}),
	)

	spouseBox.Add(spouseHeaderRow1)
	spouseBox.Add(spouseHeaderRow2)

	if fv.currentSpouse == nil {
		spouseBox.Add(widget.NewLabel("No spouse recorded"))
	} else {
		spouseBox.Add(fv.makeSpouseCardWithMarriage(*fv.currentSpouse, fv.currentMarriage))
	}

	// --- CHILDREN SECTION (FILTERED BY CURRENT MARRIAGE) ---
	childrenBox := container.NewVBox(
		widget.NewLabelWithStyle("Children", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	// Get children for the current marriage only
	var children []store.Person
	if fv.currentSpouse != nil {
		children, _ = fv.store.GetChildrenOfParents(p.ID, fv.currentSpouse.ID)
	} else {
		// No spouse, show all children
		children, _ = fv.store.GetRelatedPeople(p.ID, "child")
	}

	if len(children) == 0 {
		childrenBox.Add(widget.NewLabel("No children recorded"))
	} else {
		// Sort children by birth date (oldest first)
		sort.Slice(children, func(i, j int) bool {
			date1 := parseDateForSort(children[i].BirthDate)
			date2 := parseDateForSort(children[j].BirthDate)

			if !date1.IsZero() && !date2.IsZero() {
				return date1.Before(date2)
			}
			if !date1.IsZero() && date2.IsZero() {
				return true
			}
			if date1.IsZero() && !date2.IsZero() {
				return false
			}
			return false
		})

		for i, child := range children {
			childCard := container.NewHBox(
				widget.NewLabel(fmt.Sprintf("%d.", i+1)),
				fv.makePersonCard(child, true),
			)
			childrenBox.Add(childCard)
		}
	}

	// PAF-style Layout: Parents on right, Person and Spouse on left, Children below
	topRow := container.New(layout.NewGridLayout(2),
		container.NewVBox(personBox, widget.NewSeparator(), spouseBox),
		parentsBox)

	fv.content.Add(topRow)
	fv.content.Add(widget.NewSeparator())
	fv.content.Add(childrenBox)

	fv.content.Refresh()
}

// makePersonCard creates a clickable card for a person with basic info.
func (fv *FamilyView) makePersonCard(p store.Person, clickable bool) fyne.CanvasObject {
	nameText := formatPersonName(p)
	if p.Gender != "" {
		nameText = fmt.Sprintf("%s (%s)", nameText, p.Gender)
	}

	// Check if person has media and add indicator
	if media, err := fv.store.GetMediaForPerson(p.ID); err == nil && len(media) > 0 {
		nameText = "📷 " + nameText
	}

	// Add bookmark indicator if bookmarked
	if p.Bookmarked {
		nameText = "★ " + nameText
	}

	// Add todo indicator if person has pending todos
	if count, err := fv.store.CountPendingTodosForPerson(p.ID); err == nil && count > 0 {
		nameText = "📝 " + nameText
	}

	// Add source indicator if person has citations
	if count, err := fv.store.CountCitationsForPerson(p.ID); err == nil && count > 0 {
		nameText = "📚 " + nameText
	}

	// Add research log indicator if person has research logs
	if count, err := fv.store.CountResearchLogsForPerson(p.ID); err == nil && count > 0 {
		nameText = "🔍 " + nameText
	}

	dateInfo := ""
	if p.BirthDate != "" {
		dateInfo = p.BirthDate
		if p.BirthPlace != "" {
			dateInfo += " - " + p.BirthPlace
		}
	}
	if p.DeathDate != "" {
		if dateInfo != "" {
			dateInfo += "\n"
		}
		dateInfo += p.DeathDate
		if p.DeathPlace != "" {
			dateInfo += " - " + p.DeathPlace
		}
	} else if p.IsLiving && dateInfo != "" {
		dateInfo += "\nLiving"
	}

	nameLabel := widget.NewLabel(nameText)
	nameLabel.TextStyle.Bold = true

	// Add alternate names if any
	cardObjects := []fyne.CanvasObject{nameLabel}
	if altNames, err := fv.store.GetAlternateNames(p.ID); err == nil && len(altNames) > 0 {
		var altNamesText []string
		for _, alt := range altNames {
			altName := fmt.Sprintf("%s %s", alt.GivenName, alt.Surname)
			if alt.GivenName == "" {
				altName = alt.Surname
			} else if alt.Surname == "" {
				altName = alt.GivenName
			}
			altNamesText = append(altNamesText, altName)
		}
		altLabel := widget.NewLabel("Also known as: " + strings.Join(altNamesText, ", "))
		altLabel.TextStyle.Italic = true
		altLabel.Wrapping = fyne.TextWrapWord
		cardObjects = append(cardObjects, altLabel)
	}

	infoLabel := widget.NewLabel(dateInfo)
	infoLabel.TextStyle.Italic = true
	infoLabel.Wrapping = fyne.TextWrapWord
	cardObjects = append(cardObjects, infoLabel)

	card := container.NewVBox(cardObjects...)

	if clickable {
		// Create a card with context menu support
		bgRect := canvas.NewRectangle(theme.InputBackgroundColor())
		cardWithBg := container.NewStack(bgRect, container.NewPadded(card))

		personPtr := &p // Capture person for context menu
		tappable := NewTappableContainer(
			cardWithBg,
			func() {
				// Left-click: Navigate to person
				if fv.onNavigate != nil {
					fv.onNavigate(p.ID)
				}
			},
			func(e *fyne.PointEvent) {
				// Right-click: Show context menu
				ShowPersonContextMenu(personPtr, fv.store, fv.window, e.AbsolutePosition, fv.onNavigate, func() {
					fv.refresh()
					fv.Refresh()
				})
			},
		)
		return tappable
	}

	return card
}

// makeCurrentPersonCard creates the card for the currently displayed person (with edit button).
func (fv *FamilyView) makeCurrentPersonCard(p store.Person) fyne.CanvasObject {
	nameText := formatPersonName(p)
	if p.Gender != "" {
		nameText += fmt.Sprintf(" (%s)", p.Gender)
	}

	// Check if person has media
	personMedia, _ := fv.store.GetMediaForPerson(p.ID)
	hasMedia := len(personMedia) > 0
	if hasMedia {
		nameText = "📷 " + nameText
	}

	// Add bookmark indicator if bookmarked
	if p.Bookmarked {
		nameText = "★ " + nameText
	}

	// Add todo indicator if person has pending todos
	if count, _ := fv.store.CountPendingTodosForPerson(p.ID); count > 0 {
		nameText = "📝 " + nameText
	}

	// Add source indicator if person has citations
	if count, _ := fv.store.CountCitationsForPerson(p.ID); count > 0 {
		nameText = "📚 " + nameText
	}

	// Add research log indicator if person has research logs
	if count, _ := fv.store.CountResearchLogsForPerson(p.ID); count > 0 {
		nameText = "🔍 " + nameText
	}

	nameLabel := widget.NewLabel(nameText)
	nameLabel.TextStyle.Bold = true

	// Add alternate names if any
	cardObjects := []fyne.CanvasObject{nameLabel}
	if altNames, err := fv.store.GetAlternateNames(p.ID); err == nil && len(altNames) > 0 {
		var altNamesText []string
		for _, alt := range altNames {
			altName := fmt.Sprintf("%s %s", alt.GivenName, alt.Surname)
			if alt.GivenName == "" {
				altName = alt.Surname
			} else if alt.Surname == "" {
				altName = alt.GivenName
			}
			altNamesText = append(altNamesText, altName)
		}
		altLabel := widget.NewLabel("Also known as: " + strings.Join(altNamesText, ", "))
		altLabel.TextStyle.Italic = true
		altLabel.Wrapping = fyne.TextWrapWord
		cardObjects = append(cardObjects, altLabel)
	}

	details := []string{}
	if p.BirthDate != "" {
		bd := "Birth: " + p.BirthDate
		if p.BirthPlace != "" {
			bd += " - " + p.BirthPlace
		}
		details = append(details, bd)
	}
	if p.DeathDate != "" {
		dd := "Death: " + p.DeathDate
		if p.DeathPlace != "" {
			dd += " - " + p.DeathPlace
		}
		details = append(details, dd)
	} else if p.IsLiving {
		details = append(details, "Living")
	}
	if p.UID != "" {
		details = append(details, "UID: "+p.UID)
	}
	if p.Notes != "" {
		details = append(details, "Notes: "+p.Notes)
	}

	detailText := strings.Join(details, "\n")
	detailLabel := widget.NewLabel(detailText)
	detailLabel.Wrapping = fyne.TextWrapWord

	editBtn := widget.NewButton("Edit Individual", func() {
		showPersonDialog(fv.window, fv.store, &p, func() {
			// Reload person after edit
			if updated, err := fv.store.GetPersonByID(p.ID); err == nil {
				fv.SetPerson(updated)
			}
		})
	})

	addParentBtn := widget.NewButton("Add Parent", func() {
		fv.addRelationshipDialog("parent")
	})

	addSpouseBtn := widget.NewButton("Add Spouse", func() {
		fv.addSpouseDialog()
	})

	addChildBtn := widget.NewButton("Add Child", func() {
		fv.addChildDialog()
	})

	deleteBtn := widget.NewButton("Delete Person", func() {
		fv.deletePersonDialog(&p)
	})

	buttons := container.NewHBox(editBtn, addParentBtn, addSpouseBtn, addChildBtn, deleteBtn)

	// Build final card with name (and alternate names if any) + details + buttons
	cardObjects = append(cardObjects, detailLabel, buttons)
	card := container.NewVBox(cardObjects...)

	// Wrap in tappable container to support context menu (right-click)
	personPtr := &p // Capture person for context menu
	tappable := NewTappableContainer(
		card,
		nil, // No left-click action (already focused)
		func(e *fyne.PointEvent) {
			// Right-click: Show context menu
			ShowPersonContextMenu(personPtr, fv.store, fv.window, e.AbsolutePosition, fv.onNavigate, func() {
				fv.refresh()
				fv.Refresh()
			})
		},
	)
	return tappable
}

// makeSpouseCard creates a card for a spouse with marriage info.
func (fv *FamilyView) makeSpouseCard(si store.SpouseInfo) fyne.CanvasObject {
	p := si.Person
	nameText := formatPersonName(p)

	// Check if spouse has media and add indicator
	if media, err := fv.store.GetMediaForPerson(p.ID); err == nil && len(media) > 0 {
		nameText = "📷 " + nameText
	}

	// Add bookmark indicator if bookmarked
	if p.Bookmarked {
		nameText = "★ " + nameText
	}

	// Add todo indicator if person has pending todos
	if count, _ := fv.store.CountPendingTodosForPerson(p.ID); count > 0 {
		nameText = "📝 " + nameText
	}

	// Add source indicator if person has citations
	if count, _ := fv.store.CountCitationsForPerson(p.ID); count > 0 {
		nameText = "📚 " + nameText
	}

	// Add research log indicator if person has research logs
	if count, _ := fv.store.CountResearchLogsForPerson(p.ID); count > 0 {
		nameText = "🔍 " + nameText
	}

	// Build marriage info with all dates
	var marriageInfo []string
	if si.MarriageDate != "" {
		marr := "Marriage: " + si.MarriageDate
		if si.MarriagePlace != "" {
			marr += " - " + si.MarriagePlace
		}
		marriageInfo = append(marriageInfo, marr)
	}

	// Show divorce/separation if applicable
	if si.DivorceDate != "" {
		div := "Divorce: " + si.DivorceDate
		marriageInfo = append(marriageInfo, div)
	} else if si.SeparationDate != "" {
		sep := "Separation: " + si.SeparationDate
		marriageInfo = append(marriageInfo, sep)
	} else if si.EndReason != "" {
		// Show end reason if no specific date (e.g., "death", "annulment")
		marriageInfo = append(marriageInfo, "Ended: "+si.EndReason)
	}

	dateInfo := ""
	if p.BirthDate != "" {
		dateInfo = "b. " + p.BirthDate
		if p.BirthPlace != "" {
			dateInfo += " - " + p.BirthPlace
		}
	}
	if p.DeathDate != "" {
		if dateInfo != "" {
			dateInfo += "\n"
		}
		dateInfo += "d. " + p.DeathDate
		if p.DeathPlace != "" {
			dateInfo += " - " + p.DeathPlace
		}
	}

	allInfo := strings.Join(marriageInfo, "\n")
	if dateInfo != "" {
		if allInfo != "" {
			allInfo += "\n"
		}
		allInfo += dateInfo
	}

	// Create card content
	nameLabel := widget.NewLabel(nameText)
	nameLabel.TextStyle.Bold = true
	infoLabel := widget.NewLabel(allInfo)
	infoLabel.Wrapping = fyne.TextWrapWord
	card := container.NewVBox(nameLabel, infoLabel)

	// Create clickable container with context menu
	bgRect := canvas.NewRectangle(theme.InputBackgroundColor())
	cardWithBg := container.NewStack(bgRect, container.NewPadded(card))

	personPtr := &p // Capture person for context menu
	tappable := NewTappableContainer(
		cardWithBg,
		func() {
			// Left-click: Navigate to person
			if fv.onNavigate != nil {
				fv.onNavigate(p.ID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Show context menu
			ShowPersonContextMenu(personPtr, fv.store, fv.window, e.AbsolutePosition, fv.onNavigate, func() {
				fv.refresh()
				fv.Refresh()
			})
		},
	)

	return tappable
}

// makeSpouseCardWithMarriage creates a card for a spouse with marriage info
func (fv *FamilyView) makeSpouseCardWithMarriage(spouse store.Person, marriage *store.SpouseInfo) fyne.CanvasObject {
	if marriage == nil {
		// No marriage info, just show basic person info
		return fv.makePersonCard(spouse, true)
	}

	// Use existing makeSpouseCard logic
	si := store.SpouseInfo{
		Spouse:         spouse,
		Person:         spouse,
		MarriageDate:   marriage.MarriageDate,
		MarriagePlace:  marriage.MarriagePlace,
		DivorceDate:    marriage.DivorceDate,
		SeparationDate: marriage.SeparationDate,
		EndReason:      marriage.EndReason,
	}
	return fv.makeSpouseCard(si)
}

// showOtherMarriagesDialog displays a dialog to select which marriage to view
func (fv *FamilyView) showOtherMarriagesDialog(allSpouses []store.SpouseInfo) {
	// Create a list widget to show all marriages
	var items []string
	for _, si := range allSpouses {
		item := formatPersonName(si.Spouse)
		if si.MarriageDate != "" {
			item += fmt.Sprintf(" - Marriage: %s", si.MarriageDate)
		}
		if si.EndReason != "" {
			item += fmt.Sprintf(" (%s)", si.EndReason)
		} else if si.DivorceDate != "" {
			item += " (Divorced)"
		}
		items = append(items, item)
	}

	list := widget.NewList(
		func() int {
			return len(items)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(items[id])
		},
	)

	var selectedID int = -1
	list.OnSelected = func(id widget.ListItemID) {
		selectedID = int(id)
	}

	// Highlight current marriage
	for i, si := range allSpouses {
		if fv.currentSpouse != nil && si.Spouse.ID == fv.currentSpouse.ID {
			list.Select(widget.ListItemID(i))
			break
		}
	}

	content := container.NewBorder(
		widget.NewLabel("Select which marriage to view:"),
		nil, nil, nil,
		container.NewScroll(list),
	)

	d := dialog.NewCustom("Other Marriages", "Close", content, fv.window)
	d.Resize(fyne.NewSize(500, 300))

	list.OnSelected = func(id widget.ListItemID) {
		selectedID = int(id)
		// Switch to selected marriage
		fv.currentMarriage = &allSpouses[selectedID]
		fv.currentSpouse = &allSpouses[selectedID].Spouse
		fv.refresh()
		d.Hide()
	}

	d.Show()
}

// addRelationshipDialog shows a dialog to add a parent or child relationship.
func (fv *FamilyView) addRelationshipDialog(relType string) {
	if fv.currentPerson == nil {
		return
	}

	// Get all people to select from
	people, err := fv.store.GetPeople()
	if err != nil {
		dialog.ShowError(err, fv.window)
		return
	}

	// Filter to exclude current person
	var availablePeople []store.Person
	for _, p := range people {
		if p.ID != fv.currentPerson.ID {
			availablePeople = append(availablePeople, p)
		}
	}

	title := fmt.Sprintf("Add %s", strings.Title(relType))
	showPersonSelectionDialog(fv.window, fv.store, availablePeople, title, func(selectedID int64) {
		// Create the relationship
		rel := &store.Relationship{
			SubjectID: fv.currentPerson.ID,
			ObjectID:  selectedID,
			Type:      relType,
		}
		if err := fv.store.CreateRelationship(rel); err != nil {
			dialog.ShowError(err, fv.window)
			return
		}

		// Record for undo
		person1, _ := fv.store.GetPersonByID(fv.currentPerson.ID)
		person2, _ := fv.store.GetPersonByID(selectedID)
		desc := fmt.Sprintf("Add %s: %s → %s", relType, formatPersonName(*person1), formatPersonName(*person2))
		RecordAddRelationship(relType, fv.currentPerson.ID, selectedID, desc)

		fv.refresh()
	}, func(newPerson *store.Person) {
		// Callback when new person is created - automatically link them
		rel := &store.Relationship{
			SubjectID: fv.currentPerson.ID,
			ObjectID:  newPerson.ID,
			Type:      relType,
		}
		if err := fv.store.CreateRelationship(rel); err != nil {
			dialog.ShowError(err, fv.window)
			return
		}

		// Record for undo
		person1, _ := fv.store.GetPersonByID(fv.currentPerson.ID)
		desc := fmt.Sprintf("Add %s: %s → %s", relType, formatPersonName(*person1), formatPersonName(*newPerson))
		RecordAddRelationship(relType, fv.currentPerson.ID, newPerson.ID, desc)

		dialog.ShowInformation("Success",
			fmt.Sprintf("%s created and linked successfully!", strings.Title(relType)), fv.window)
		fv.refresh()
	})
}

// showPersonSelectionDialog shows a searchable dialog to select or create a person.
func showPersonSelectionDialog(w fyne.Window, s *store.Store, availablePeople []store.Person,
	title string, onSelect func(int64), onCreateAndLink func(*store.Person)) {

	// Filtered list
	filteredPeople := make([]store.Person, len(availablePeople))
	copy(filteredPeople, availablePeople)

	var selectedIndex int = -1

	// Search entry
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search by name...")

	// Person list
	personList := widget.NewList(
		func() int { return len(filteredPeople) },
		func() fyne.CanvasObject { return widget.NewLabel("genealogy") },
		func(i int, o fyne.CanvasObject) {
			if i >= len(filteredPeople) {
				return
			}
			p := filteredPeople[i]
			label := o.(*widget.Label)
			text := formatPersonName(p)
			if p.BirthDate != "" {
				text += fmt.Sprintf(" (b. %s)", p.BirthDate)
			}
			label.SetText(text)
		},
	)

	personList.OnSelected = func(i int) {
		selectedIndex = i
	}

	// Update filter function
	updateFilter := func(searchText string) {
		filteredPeople = filteredPeople[:0]
		searchText = strings.ToLower(strings.TrimSpace(searchText))
		for _, p := range availablePeople {
			if searchText == "" ||
				strings.Contains(strings.ToLower(p.GivenName), searchText) ||
				strings.Contains(strings.ToLower(p.Surname), searchText) {
				filteredPeople = append(filteredPeople, p)
			}
		}
		selectedIndex = -1
		personList.UnselectAll()
		personList.Refresh()
	}

	searchEntry.OnChanged = updateFilter

	// Declare dialog reference (will be set below)
	var d dialog.Dialog

	// Create new person button
	createNewBtn := widget.NewButton("Create New Person", func() {
		// Show person dialog, then call onCreateAndLink
		showPersonDialog(w, s, nil, func() {
			// Get the most recently created person
			people, err := s.GetPeople()
			if err != nil || len(people) == 0 {
				return
			}
			// The newest person should be the last one (highest ID)
			var newest *store.Person
			for i := range people {
				if newest == nil || people[i].ID > newest.ID {
					newest = &people[i]
				}
			}
			if newest != nil && onCreateAndLink != nil {
				onCreateAndLink(newest)
				// Close the selection dialog after successfully creating and linking
				if d != nil {
					d.Hide()
				}
			}
		})
	})

	// Layout
	listContainer := container.NewBorder(searchEntry, createNewBtn, nil, nil, personList)
	listContainer.Resize(fyne.NewSize(400, 400))

	d = dialog.NewCustomConfirm(title, "Link Selected", "Cancel", listContainer, func(ok bool) {
		if !ok {
			return
		}
		if selectedIndex >= 0 && selectedIndex < len(filteredPeople) {
			if onSelect != nil {
				onSelect(filteredPeople[selectedIndex].ID)
			}
		} else {
			dialog.ShowInformation("Selection Required", "Please select a person or create a new one", w)
		}
	}, w)

	d.Resize(fyne.NewSize(450, 500))
	d.Show()
}

// showPersonSelectionDialogWithMarriage shows a searchable dialog with marriage fields.
func showPersonSelectionDialogWithMarriage(w fyne.Window, s *store.Store, availablePeople []store.Person,
	relationshipType *widget.Select, marriageDate, marriagePlace, divorceDate, separationDate *widget.Entry,
	endReason *widget.Select, onSelect func(int64), onCreateAndLink func(*store.Person)) {

	// Filtered list
	filteredPeople := make([]store.Person, len(availablePeople))
	copy(filteredPeople, availablePeople)

	var selectedIndex int = -1

	// Search entry
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search by name...")

	// Person list
	personList := widget.NewList(
		func() int { return len(filteredPeople) },
		func() fyne.CanvasObject { return widget.NewLabel("genealogy") },
		func(i int, o fyne.CanvasObject) {
			if i >= len(filteredPeople) {
				return
			}
			p := filteredPeople[i]
			label := o.(*widget.Label)
			text := formatPersonName(p)
			if p.BirthDate != "" {
				text += fmt.Sprintf(" (b. %s)", p.BirthDate)
			}
			label.SetText(text)
		},
	)

	personList.OnSelected = func(i int) {
		selectedIndex = i
	}

	// Update filter function
	updateFilter := func(searchText string) {
		filteredPeople = filteredPeople[:0]
		searchText = strings.ToLower(strings.TrimSpace(searchText))
		for _, p := range availablePeople {
			if searchText == "" ||
				strings.Contains(strings.ToLower(p.GivenName), searchText) ||
				strings.Contains(strings.ToLower(p.Surname), searchText) {
				filteredPeople = append(filteredPeople, p)
			}
		}
		selectedIndex = -1
		personList.UnselectAll()
		personList.Refresh()
	}

	searchEntry.OnChanged = updateFilter

	// Declare dialog reference (will be set below)
	var d dialog.Dialog

	// Create new person button
	createNewBtn := widget.NewButton("Create New Person", func() {
		showPersonDialog(w, s, nil, func() {
			// Get the most recently created person
			people, err := s.GetPeople()
			if err != nil || len(people) == 0 {
				return
			}
			var newest *store.Person
			for i := range people {
				if newest == nil || people[i].ID > newest.ID {
					newest = &people[i]
				}
			}
			if newest != nil && onCreateAndLink != nil {
				onCreateAndLink(newest)
				// Close the selection dialog after successfully creating and linking
				if d != nil {
					d.Hide()
				}
			}
		})
	})

	// Marriage/Relationship info section
	marriageSection := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Relationship Information:"),
		container.NewVBox(
			widget.NewLabel("Relationship Type:"), relationshipType,
			widget.NewLabel("Union/Marriage Date:"), marriageDate,
			widget.NewLabel("Union/Marriage Place:"), marriagePlace,
		),
		widget.NewSeparator(),
		widget.NewLabel("End of Relationship (if applicable):"),
		container.NewVBox(
			widget.NewLabel("Divorce Date:"), divorceDate,
			widget.NewLabel("Separation Date:"), separationDate,
			widget.NewLabel("End Reason:"), endReason,
		),
	)

	// Layout
	topSection := container.NewBorder(searchEntry, nil, nil, nil, personList)
	mainContent := container.NewBorder(topSection, container.NewVBox(createNewBtn, marriageSection), nil, nil, nil)
	mainContent.Resize(fyne.NewSize(400, 500))

	d = dialog.NewCustomConfirm("Add Spouse", "Link Selected", "Cancel", mainContent, func(ok bool) {
		if !ok {
			return
		}
		if selectedIndex >= 0 && selectedIndex < len(filteredPeople) {
			if onSelect != nil {
				onSelect(filteredPeople[selectedIndex].ID)
			}
		} else {
			dialog.ShowInformation("Selection Required", "Please select a person or create a new one", w)
		}
	}, w)

	d.Resize(fyne.NewSize(500, 700))
	d.Show()
}

// addChildDialog shows a dialog to add a child, with smart linking to both parents if married.
func (fv *FamilyView) addChildDialog() {
	if fv.currentPerson == nil {
		return
	}

	// Check if current person has spouse(s)
	spouses, _ := fv.store.GetSpouses(fv.currentPerson.ID)

	// If multiple spouses, let user select which marriage this child belongs to
	if len(spouses) > 1 {
		fv.selectSpouseForChild(spouses)
		return
	}

	// Single spouse or no spouse - proceed with standard flow
	fv.addChildWithSpouse(spouses)
}

// selectSpouseForChild shows a dialog to select which spouse/marriage a child belongs to.
func (fv *FamilyView) selectSpouseForChild(spouses []store.SpouseInfo) {
	if fv.currentPerson == nil {
		return
	}

	// Build options for each marriage
	opts := []string{}
	for i, si := range spouses {
		label := fmt.Sprintf("%d. %s", i+1, formatPersonName(si.Person))
		if si.MarriageDate != "" {
			label += fmt.Sprintf(" (m. %s", si.MarriageDate)
			if si.DivorceDate != "" {
				label += fmt.Sprintf(", div. %s", si.DivorceDate)
			} else if si.SeparationDate != "" {
				label += fmt.Sprintf(", sep. %s", si.SeparationDate)
			}
			label += ")"
		}
		opts = append(opts, label)
	}
	opts = append(opts, fmt.Sprintf("Child of %s only (no other parent)", fv.currentPerson.GivenName))

	sel := widget.NewSelect(opts, func(string) {})
	sel.SetSelected(opts[0])

	dialog.ShowCustomConfirm("Select Marriage", "Continue", "Cancel",
		container.NewVBox(
			widget.NewLabel("This person has multiple marriages."),
			widget.NewLabel("Select which marriage this child belongs to:"),
			sel,
		),
		func(ok bool) {
			if !ok {
				return
			}
			selectedIdx := -1
			for i, opt := range opts {
				if opt == sel.Selected {
					selectedIdx = i
					break
				}
			}

			if selectedIdx == len(opts)-1 {
				// Child of current person only
				fv.addChildWithSpouse(nil)
			} else if selectedIdx >= 0 && selectedIdx < len(spouses) {
				// Child of selected marriage
				fv.addChildWithSpouse([]store.SpouseInfo{spouses[selectedIdx]})
			}
		}, fv.window)
}

// addChildWithSpouse adds a child and links to current person and specified spouse(s).
func (fv *FamilyView) addChildWithSpouse(spouses []store.SpouseInfo) {
	if fv.currentPerson == nil {
		return
	}

	// Get all people to select from
	people, err := fv.store.GetPeople()
	if err != nil {
		dialog.ShowError(err, fv.window)
		return
	}

	// Filter to exclude current person
	var availablePeople []store.Person
	for _, p := range people {
		if p.ID != fv.currentPerson.ID {
			availablePeople = append(availablePeople, p)
		}
	}

	title := "Add Child"
	showPersonSelectionDialog(fv.window, fv.store, availablePeople, title, func(selectedID int64) {
		// Create relationship to current person
		rel := &store.Relationship{
			SubjectID: fv.currentPerson.ID,
			ObjectID:  selectedID,
			Type:      "child",
		}
		if err := fv.store.CreateRelationship(rel); err != nil {
			dialog.ShowError(err, fv.window)
			return
		}

		// Record for undo
		child, _ := fv.store.GetPersonByID(selectedID)
		desc := fmt.Sprintf("Add child: %s → %s", formatPersonName(*fv.currentPerson), formatPersonName(*child))
		RecordAddRelationship("child", fv.currentPerson.ID, selectedID, desc)

		// Link child to selected spouse(s)
		for _, si := range spouses {
			spouseRel := &store.Relationship{
				SubjectID: si.Person.ID,
				ObjectID:  selectedID,
				Type:      "child",
			}
			if err := fv.store.CreateRelationship(spouseRel); err == nil {
				// Record for undo
				desc2 := fmt.Sprintf("Add child: %s → %s", formatPersonName(si.Person), formatPersonName(*child))
				RecordAddRelationship("child", si.Person.ID, selectedID, desc2)
			}
		}

		fv.refresh()
	}, func(newPerson *store.Person) {
		// Callback when new person is created - link to current person
		rel := &store.Relationship{
			SubjectID: fv.currentPerson.ID,
			ObjectID:  newPerson.ID,
			Type:      "child",
		}
		if err := fv.store.CreateRelationship(rel); err != nil {
			dialog.ShowError(err, fv.window)
			return
		}

		// Record for undo
		desc := fmt.Sprintf("Add child: %s → %s", formatPersonName(*fv.currentPerson), formatPersonName(*newPerson))
		RecordAddRelationship("child", fv.currentPerson.ID, newPerson.ID, desc)

		// Link child to selected spouse(s)
		parentNames := []string{formatPersonName(*fv.currentPerson)}
		for _, si := range spouses {
			spouseRel := &store.Relationship{
				SubjectID: si.Person.ID,
				ObjectID:  newPerson.ID,
				Type:      "child",
			}
			if err := fv.store.CreateRelationship(spouseRel); err == nil {
				// Record for undo
				desc2 := fmt.Sprintf("Add child: %s → %s", formatPersonName(si.Person), formatPersonName(*newPerson))
				RecordAddRelationship("child", si.Person.ID, newPerson.ID, desc2)
			}
			parentNames = append(parentNames, formatPersonName(si.Person))
		}

		if len(spouses) > 0 {
			dialog.ShowInformation("Success",
				fmt.Sprintf("Child created and linked to:\n%s", strings.Join(parentNames, " and ")),
				fv.window)
		} else {
			dialog.ShowInformation("Success", "Child created and linked successfully!", fv.window)
		}
		fv.refresh()
	})
}

// addSpouseDialog shows a dialog to add a spouse with marriage information.
func (fv *FamilyView) addSpouseDialog() {
	if fv.currentPerson == nil {
		return
	}

	// Get all people to select from
	people, err := fv.store.GetPeople()
	if err != nil {
		dialog.ShowError(err, fv.window)
		return
	}

	// Filter to exclude current person
	var availablePeople []store.Person
	for _, p := range people {
		if p.ID != fv.currentPerson.ID {
			availablePeople = append(availablePeople, p)
		}
	}

	// Relationship type
	relationshipType := widget.NewSelect([]string{"spouse", "partner", "cohabitation", "other"}, func(string) {})
	relationshipType.SetSelected("spouse") // Default to spouse

	// Marriage/Union info fields
	marriageDate := widget.NewEntry()
	marriageDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")
	marriagePlaceAutocomplete := NewPlaceAutocompleteContainer(func() *store.Store { return fv.store }, "")

	// End of marriage fields
	divorceDate := widget.NewEntry()
	divorceDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")
	separationDate := widget.NewEntry()
	separationDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")
	endReason := widget.NewSelect([]string{"", "divorce", "separation", "death", "annulment", "other"}, func(string) {})

	showPersonSelectionDialogWithMarriage(fv.window, fv.store, availablePeople,
		relationshipType, marriageDate, marriagePlaceAutocomplete.Entry, divorceDate, separationDate, endReason,
		func(selectedID int64) {
			// Create relationship with union info
			rel := &store.Relationship{
				SubjectID:      fv.currentPerson.ID,
				ObjectID:       selectedID,
				Type:           relationshipType.Selected,
				MarriageDate:   strings.TrimSpace(marriageDate.Text),
				MarriagePlace:  strings.TrimSpace(marriagePlaceAutocomplete.GetText()),
				DivorceDate:    strings.TrimSpace(divorceDate.Text),
				SeparationDate: strings.TrimSpace(separationDate.Text),
				EndReason:      endReason.Selected,
			}
			if err := fv.store.CreateRelationship(rel); err != nil {
				dialog.ShowError(err, fv.window)
				return
			}

			// Record for undo
			partner, _ := fv.store.GetPersonByID(selectedID)
			desc := fmt.Sprintf("Add %s: %s ↔ %s", relationshipType.Selected, formatPersonName(*fv.currentPerson), formatPersonName(*partner))
			RecordAddRelationship(relationshipType.Selected, fv.currentPerson.ID, selectedID, desc)

			fv.refresh()
		}, func(newPerson *store.Person) {
			// Callback when new person is created - automatically link as partner/spouse
			rel := &store.Relationship{
				SubjectID:      fv.currentPerson.ID,
				ObjectID:       newPerson.ID,
				Type:           relationshipType.Selected,
				MarriageDate:   strings.TrimSpace(marriageDate.Text),
				MarriagePlace:  strings.TrimSpace(marriagePlaceAutocomplete.GetText()),
				DivorceDate:    strings.TrimSpace(divorceDate.Text),
				SeparationDate: strings.TrimSpace(separationDate.Text),
				EndReason:      endReason.Selected,
			}
			if err := fv.store.CreateRelationship(rel); err != nil {
				dialog.ShowError(err, fv.window)
				return
			}

			// Record for undo
			desc := fmt.Sprintf("Add %s: %s ↔ %s", relationshipType.Selected, formatPersonName(*fv.currentPerson), formatPersonName(*newPerson))
			RecordAddRelationship(relationshipType.Selected, fv.currentPerson.ID, newPerson.ID, desc)

			dialog.ShowInformation("Success", "Partner created and linked successfully!", fv.window)
			fv.refresh()
		})
}

// showEditMarriageDialog displays a dialog to edit marriage/relationship details
func (fv *FamilyView) showEditMarriageDialog(marriage *store.SpouseInfo, spouseID int64) {
	if marriage == nil || fv.currentPerson == nil {
		return
	}

	// Create form fields pre-populated with current values
	relationshipType := widget.NewSelect([]string{"spouse", "partner", "cohabitation", "other"}, func(string) {})
	relationshipType.SetSelected("spouse") // Default

	marriageDate := widget.NewEntry()
	marriageDate.SetText(marriage.MarriageDate)
	marriageDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")

	marriagePlaceAutocomplete := NewPlaceAutocompleteContainer(func() *store.Store { return fv.store }, marriage.MarriagePlace)

	divorceDate := widget.NewEntry()
	divorceDate.SetText(marriage.DivorceDate)
	divorceDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")

	separationDate := widget.NewEntry()
	separationDate.SetText(marriage.SeparationDate)
	separationDate.SetPlaceHolder("YYYY-MM-DD or DD MMM YYYY")

	endReason := widget.NewSelect([]string{"", "divorce", "separation", "death", "annulment", "other"}, func(string) {})
	if marriage.EndReason != "" {
		endReason.SetSelected(marriage.EndReason)
	}

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Relationship Type", Widget: relationshipType},
			{Text: "Marriage/Union Date", Widget: marriageDate},
			{Text: "Marriage/Union Place", Widget: marriagePlaceAutocomplete.Container},
			{Text: "Divorce Date", Widget: divorceDate},
			{Text: "Separation Date", Widget: separationDate},
			{Text: "Ended By", Widget: endReason},
		},
		OnSubmit: func() {
			// Update the relationship in the database
			// Find the existing relationship (try "spouse" type first, then "partner")
			var rels []store.Relationship
			var err error

			// Try to find any spouse/partner relationship
			for _, relType := range []string{"spouse", "partner", "cohabitation"} {
				rels, err = fv.store.GetRelationshipsBetween(fv.currentPerson.ID, spouseID, relType)
				if err == nil && len(rels) > 0 {
					break
				}
			}

			if err != nil || len(rels) == 0 {
				dialog.ShowError(fmt.Errorf("Could not find relationship to update"), fv.window)
				return
			}

			rel := rels[0]
			rel.MarriageDate = strings.TrimSpace(marriageDate.Text)
			rel.MarriagePlace = strings.TrimSpace(marriagePlaceAutocomplete.GetText())
			rel.DivorceDate = strings.TrimSpace(divorceDate.Text)
			rel.SeparationDate = strings.TrimSpace(separationDate.Text)
			rel.EndReason = endReason.Selected
			// Note: relationship type change would require deleting and recreating

			if err := fv.store.UpdateRelationship(&rel); err != nil {
				dialog.ShowError(err, fv.window)
				return
			}

			// Also update the reverse relationship
			reverseRels, err := fv.store.GetRelationshipsBetween(spouseID, fv.currentPerson.ID, rel.Type)
			if err == nil && len(reverseRels) > 0 {
				reverseRel := reverseRels[0]
				reverseRel.MarriageDate = rel.MarriageDate
				reverseRel.MarriagePlace = rel.MarriagePlace
				reverseRel.DivorceDate = rel.DivorceDate
				reverseRel.SeparationDate = rel.SeparationDate
				reverseRel.EndReason = rel.EndReason
				_ = fv.store.UpdateRelationship(&reverseRel)
			}

			dialog.ShowInformation("Success", "Marriage details updated successfully!", fv.window)

			// Update the current marriage info and refresh view
			marriage.MarriageDate = rel.MarriageDate
			marriage.MarriagePlace = rel.MarriagePlace
			marriage.DivorceDate = rel.DivorceDate
			marriage.SeparationDate = rel.SeparationDate
			marriage.EndReason = rel.EndReason
			fv.refresh()
		},
		OnCancel: func() {
			// Dialog will close automatically
		},
	}

	d := dialog.NewCustom("Edit Marriage/Relationship", "Close", form, fv.window)
	d.Resize(fyne.NewSize(500, 400))
	d.Show()
}

// parseDateForSort attempts to parse a date string into a time.Time for sorting purposes
// Supports common formats: "DD MMM YYYY", "YYYY-MM-DD", "YYYY"
func parseDateForSort(dateStr string) time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{} // Zero time
	}

	// Try common date formats
	formats := []string{
		"2 Jan 2006",  // DD MMM YYYY
		"02 Jan 2006", // DD MMM YYYY (with leading zero)
		"2006-01-02",  // YYYY-MM-DD
		"2006",        // YYYY only
		"Jan 2006",    // MMM YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	return time.Time{} // Failed to parse, return zero time
}

// deletePersonDialog shows a confirmation dialog to delete a person and their relationships
func (fv *FamilyView) deletePersonDialog(person *store.Person) {
	if person == nil {
		return
	}

	// Build warning message with relationship count
	relationships, _ := fv.store.GetRelationships()
	relCount := 0
	for _, rel := range relationships {
		if rel.SubjectID == person.ID || rel.ObjectID == person.ID {
			relCount++
		}
	}

	warningMsg := fmt.Sprintf("Are you sure you want to delete:\n\n%s\n\n",
		formatPersonName(*person))

	if relCount > 0 {
		warningMsg += fmt.Sprintf("This will also remove %d relationship(s) involving this person.\n\n", relCount)
	}

	warningMsg += "You can undo this action using Edit → Undo (Cmd/Ctrl+U)."

	dialog.ShowConfirm("Delete Person", warningMsg, func(confirmed bool) {
		if !confirmed {
			return
		}

		// Gather relationship data for undo before deleting
		spouses, _ := fv.store.GetSpouses(person.ID)
		childRels, _ := fv.store.GetRelatedPeople(person.ID, "child")
		parentRels, _ := fv.store.GetRelatedPeople(person.ID, "parent")

		var childIDs, parentIDs []int64
		for _, child := range childRels {
			childIDs = append(childIDs, child.ID)
		}
		for _, parent := range parentRels {
			parentIDs = append(parentIDs, parent.ID)
		}

		// Delete the person (database will cascade delete relationships)
		if err := fv.store.DeletePerson(person.ID); err != nil {
			dialog.ShowError(err, fv.window)
			return
		}

		// Record for undo
		RecordDeletePerson(person, spouses, childIDs, parentIDs)

		// Refresh the view - navigate to first available person
		people, err := fv.store.GetPeople()
		if err != nil || len(people) == 0 {
			// No people left, clear the view
			fv.SetPerson(nil)
			return
		}

		// Navigate to first person
		fv.SetPerson(&people[0])

		dialog.ShowInformation("Deleted",
			fmt.Sprintf("%s has been deleted successfully.", formatPersonName(*person)),
			fv.window)
	}, fv.window)
}
