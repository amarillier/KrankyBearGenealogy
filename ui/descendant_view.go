package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// DescendantView displays a person's descendants in a tree layout
type DescendantView struct {
	widget.BaseWidget
	store          *store.Store
	currentPerson  *store.Person
	window         fyne.Window
	onNavigate     func(personID int64)
	content        *fyne.Container
	chartScroll    *container.Scroll
	
	// Enhanced features
	maxGenerations   int
	colorMode        ColorMode
	zoomLevel        float32
	collapsedBranches map[int64]bool
	enhancedMode     bool // Show spouses and which children belong to which marriage
	
	// UI controls
	genSelect    *widget.Select
	colorSelect  *widget.Select
	zoomSlider   *widget.Slider
	enhancedCheck *widget.Check
}

// Global list of open descendant chart windows for dynamic synchronization
var openDescendantCharts []*DescendantView

// NewDescendantView creates a new descendant view widget
func NewDescendantView(s *store.Store, w fyne.Window, onNavigate func(personID int64)) *DescendantView {
	dv := &DescendantView{
		store:            s,
		window:           w,
		onNavigate:       onNavigate,
		maxGenerations:   4,
		colorMode:        ColorByGender,
		zoomLevel:        1.0,
		collapsedBranches: make(map[int64]bool),
	}
	dv.ExtendBaseWidget(dv)
	return dv
}

// SetPerson updates the view to show a specific person's descendants
func (dv *DescendantView) SetPerson(p *store.Person) {
	dv.currentPerson = p
	dv.refresh()
	dv.Refresh()
}

// CreateRenderer implements fyne.Widget
func (dv *DescendantView) CreateRenderer() fyne.WidgetRenderer {
	if dv.currentPerson == nil {
		dv.content = container.NewVBox(widget.NewLabel("No person selected"))
		return widget.NewSimpleRenderer(dv.content)
	}

	// Create control panel
	controlPanel := dv.createControlPanel()
	
	// Create descendant chart with scroll
	chart := dv.buildDescendantChart()
	dv.chartScroll = container.NewVScroll(chart)
	
	// Layout with control panel at top
	dv.content = container.NewBorder(controlPanel, nil, nil, nil, dv.chartScroll)
	
	return widget.NewSimpleRenderer(dv.content)
}

// createControlPanel creates the control panel with generation selector, color mode, zoom, etc.
func (dv *DescendantView) createControlPanel() fyne.CanvasObject {
	// Generation selector
	genOptions := []string{"3 Generations", "4 Generations", "5 Generations", "6 Generations", "7 Generations", "8 Generations"}
	dv.genSelect = widget.NewSelect(genOptions, func(value string) {
		switch value {
		case "3 Generations":
			dv.maxGenerations = 3
		case "4 Generations":
			dv.maxGenerations = 4
		case "5 Generations":
			dv.maxGenerations = 5
		case "6 Generations":
			dv.maxGenerations = 6
		case "7 Generations":
			dv.maxGenerations = 7
		case "8 Generations":
			dv.maxGenerations = 8
		}
		dv.refresh()
	})
	dv.genSelect.SetSelected("4 Generations")
	
	// Color mode selector
	colorOptions := []string{"No Color", "By Gender", "By Living Status", "By Data Completeness"}
	dv.colorSelect = widget.NewSelect(colorOptions, func(value string) {
		switch value {
		case "No Color":
			dv.colorMode = ColorNone
		case "By Gender":
			dv.colorMode = ColorByGender
		case "By Living Status":
			dv.colorMode = ColorByLivingStatus
		case "By Data Completeness":
			dv.colorMode = ColorByDataCompleteness
		}
		dv.refresh()
	})
	dv.colorSelect.SetSelected("By Gender")
	
	// Zoom slider
	dv.zoomSlider = widget.NewSlider(0.5, 2.0)
	dv.zoomSlider.Value = 1.0
	dv.zoomSlider.Step = 0.1
	dv.zoomSlider.OnChanged = func(value float64) {
		dv.zoomLevel = float32(value)
		dv.refresh()
	}
	zoomLabel := widget.NewLabel(fmt.Sprintf("Zoom: %.0f%%", dv.zoomLevel*100))
	dv.zoomSlider.OnChanged = func(value float64) {
		dv.zoomLevel = float32(value)
		zoomLabel.SetText(fmt.Sprintf("Zoom: %.0f%%", dv.zoomLevel*100))
		dv.refresh()
	}
	
	// Enhanced mode checkbox
	dv.enhancedCheck = widget.NewCheck("Enhanced (show spouses)", func(checked bool) {
		dv.enhancedMode = checked
		dv.refresh()
	})
	dv.enhancedCheck.SetChecked(dv.enhancedMode)
	
	// Expand/Collapse All buttons
	expandAllBtn := widget.NewButton("Expand All", func() {
		dv.collapsedBranches = make(map[int64]bool)
		dv.refresh()
	})
	
	exportBtn := widget.NewButton("Export Chart", func() {
		dv.showExportDialog()
	})
	
	// Layout controls
	controls := container.NewVBox(
		widget.NewLabel("Descendant Chart Controls"),
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabel("Generations:"),
			dv.genSelect,
			widget.NewLabel("Color:"),
			dv.colorSelect,
			dv.enhancedCheck,
		),
		container.NewHBox(
			zoomLabel,
			dv.zoomSlider,
			expandAllBtn,
			exportBtn,
		),
	)
	
	return controls
}

// refresh rebuilds the descendant chart
func (dv *DescendantView) refresh() {
	if dv.chartScroll == nil {
		return
	}
	
	// Rebuild just the chart content
	chart := dv.buildDescendantChart()
	dv.chartScroll.Content = chart
	dv.chartScroll.Refresh()
}

// buildDescendantChart creates the descendant tree layout
func (dv *DescendantView) buildDescendantChart() fyne.CanvasObject {
	person := dv.currentPerson
	
	// Build tree structure recursively
	tree := dv.buildDescendantTree(person, 1)
	
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Descendant Chart for %s (%d Generations)", formatPersonName(*person), dv.maxGenerations),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	return container.NewBorder(title, nil, nil, nil, tree)
}

// buildDescendantTree recursively builds the descendant tree
func (dv *DescendantView) buildDescendantTree(person *store.Person, generation int) fyne.CanvasObject {
	if person == nil || generation > dv.maxGenerations {
		return container.NewVBox()
	}
	
	// Check if this branch is collapsed
	isCollapsed := dv.collapsedBranches[person.ID]
	
	// Create person box
	personBox := dv.makePersonBox(person, generation == 1, generation)
	
	// If collapsed or at max generation, just return the person box
	if isCollapsed || generation >= dv.maxGenerations {
		return container.NewVBox(personBox)
	}
	
	// Get children
	children := dv.getChildren(person.ID)
	if len(children) == 0 {
		return container.NewVBox(personBox)
	}
	
	// Add spacing
	spacer := canvas.NewRectangle(theme.BackgroundColor())
	spacer.SetMinSize(fyne.NewSize(0, 20*dv.zoomLevel))
	
	// Enhanced mode: show spouses and group children by marriage
	if dv.enhancedMode {
		return dv.buildEnhancedDescendantTree(person, children, generation, personBox, spacer)
	}
	
	// Simple mode: just show all children
	var childTrees []fyne.CanvasObject
	for _, child := range children {
		childTree := dv.buildDescendantTree(&child, generation+1)
		childTrees = append(childTrees, childTree)
	}
	
	// Layout: person at top, children below in horizontal row
	childrenRow := container.NewHBox(childTrees...)
	
	return container.NewVBox(
		personBox,
		spacer,
		childrenRow,
	)
}

// buildEnhancedDescendantTree builds the tree with spouse information
func (dv *DescendantView) buildEnhancedDescendantTree(person *store.Person, children []store.Person, generation int, personBox fyne.CanvasObject, spacer fyne.CanvasObject) fyne.CanvasObject {
	// Get all spouses/marriages
	spouses, _ := dv.store.GetSpouses(person.ID)
	
	if len(spouses) == 0 {
		// No spouse info, just show children
		var childTrees []fyne.CanvasObject
		for _, child := range children {
			childTree := dv.buildDescendantTree(&child, generation+1)
			childTrees = append(childTrees, childTree)
		}
		childrenRow := container.NewHBox(childTrees...)
		return container.NewVBox(personBox, spacer, childrenRow)
	}
	
	// Group children by their parents (find which spouse is the other parent)
	marriageGroups := make(map[int64][]store.Person) // spouseID -> children
	unmarriedChildren := []store.Person{}
	
	for _, child := range children {
		// Get child's parents
		parents, _ := dv.store.GetRelatedPeople(child.ID, "parent")
		
		// Find which spouse is the other parent
		foundSpouse := false
		for _, parent := range parents {
			if parent.ID != person.ID {
				// This parent is a spouse
				for _, si := range spouses {
					if si.Spouse.ID == parent.ID {
						marriageGroups[si.Spouse.ID] = append(marriageGroups[si.Spouse.ID], child)
						foundSpouse = true
						break
					}
				}
				if foundSpouse {
					break
				}
			}
		}
		
		if !foundSpouse {
			unmarriedChildren = append(unmarriedChildren, child)
		}
	}
	
	// Build the layout: person, then for each marriage: spouse + children
	var familyGroups []fyne.CanvasObject
	
	for _, si := range spouses {
		childrenOfMarriage := marriageGroups[si.Spouse.ID]
		if len(childrenOfMarriage) == 0 {
			continue // Skip marriages with no children
		}
		
		// Create spouse box
		spouseBox := dv.makeSpouseBox(&si.Spouse, si.MarriageDate, si.EndReason)
		
		// Build child trees for this marriage
		var childTrees []fyne.CanvasObject
		for _, child := range childrenOfMarriage {
			childTree := dv.buildDescendantTree(&child, generation+1)
			childTrees = append(childTrees, childTree)
		}
		childrenRow := container.NewHBox(childTrees...)
		
		// Marriage group: spouse above, children below
		marriageGroup := container.NewVBox(
			spouseBox,
			childrenRow,
		)
		
		familyGroups = append(familyGroups, marriageGroup)
	}
	
	// Add unmarried children if any
	if len(unmarriedChildren) > 0 {
		var childTrees []fyne.CanvasObject
		for _, child := range unmarriedChildren {
			childTree := dv.buildDescendantTree(&child, generation+1)
			childTrees = append(childTrees, childTree)
		}
		childrenRow := container.NewHBox(childTrees...)
		
		unknownLabel := widget.NewLabel("(unknown spouse)")
		unknownLabel.TextStyle.Italic = true
		marriageGroup := container.NewVBox(
			unknownLabel,
			childrenRow,
		)
		familyGroups = append(familyGroups, marriageGroup)
	}
	
	// Combine all marriage groups horizontally
	allFamilies := container.NewHBox(familyGroups...)
	
	return container.NewVBox(
		personBox,
		spacer,
		allFamilies,
	)
}

// makeSpouseBox creates a smaller box for a spouse
func (dv *DescendantView) makeSpouseBox(spouse *store.Person, marriageDate, endReason string) fyne.CanvasObject {
	nameText := formatPersonName(*spouse)
	
	// Add marriage info
	infoText := ""
	if marriageDate != "" {
		infoText = "m. " + marriageDate
	}
	if endReason != "" {
		if infoText != "" {
			infoText += " "
		}
		infoText += "(" + endReason + ")"
	}
	
	nameLabel := widget.NewLabel(nameText)
	nameLabel.TextStyle.Italic = true
	nameLabel.Wrapping = fyne.TextWrapOff
	
	infoLabel := widget.NewLabel(infoText)
	infoLabel.TextStyle.Italic = true
	infoLabel.Wrapping = fyne.TextWrapOff
	
	content := container.NewVBox(nameLabel, infoLabel)
	
	// Apply zoom to box size
	boxWidth := 180 * dv.zoomLevel
	boxHeight := 60 * dv.zoomLevel
	content.Resize(fyne.NewSize(boxWidth, boxHeight))
	
	// Light background to distinguish from main person
	bgColor := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	bg := canvas.NewRectangle(bgColor)
	
	// Wrap in TappableContainer for navigation and context menu
	baseContainer := container.NewStack(bg, container.NewPadded(content))
	
	personPtr := spouse
	tap := NewTappableContainer(
		baseContainer,
		func() {
			// Left-click: Navigate to spouse
			if dv.onNavigate != nil {
				dv.onNavigate(spouse.ID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Show context menu
			ShowPersonContextMenu(personPtr, dv.store, dv.window, e.AbsolutePosition, dv.onNavigate, func() {
				dv.refresh()
				dv.Refresh()
			})
		},
	)
	
	tap.Resize(fyne.NewSize(boxWidth, boxHeight))
	
	return tap
}

// makePersonBox creates a clickable box for a person with color coding
func (dv *DescendantView) makePersonBox(p *store.Person, isRoot bool, generation int) fyne.CanvasObject {
	nameText := formatPersonName(*p)
	
	// Check if person has media and add indicator
	if media, err := dv.store.GetMediaForPerson(p.ID); err == nil && len(media) > 0 {
		nameText = "📷 " + nameText
	}
	
	// Add bookmark indicator if bookmarked
	if p.Bookmarked {
		nameText = "★ " + nameText
	}
	
	// Add todo indicator if person has pending todos
	if count, _ := dv.store.CountPendingTodosForPerson(p.ID); count > 0 {
		nameText = "📝 " + nameText
	}
	
	// Add source indicator if person has citations
	if count, _ := dv.store.CountCitationsForPerson(p.ID); count > 0 {
		nameText = "📚 " + nameText
	}

	// Add research log indicator if person has research logs
	if count, _ := dv.store.CountResearchLogsForPerson(p.ID); count > 0 {
		nameText = "🔍 " + nameText
	}
	
	// Add collapse indicator if person has children and is not at max generation
	if generation < dv.maxGenerations {
		children := dv.getChildren(p.ID)
		if len(children) > 0 {
			if dv.collapsedBranches[p.ID] {
				nameText = "▶ " + nameText
			} else {
				nameText = "▼ " + nameText
			}
		}
	}

	var dateText string
	if p.BirthDate != "" {
		dateText = "b. " + p.BirthDate
	}
	if p.DeathDate != "" {
		if dateText != "" {
			dateText += " "
		}
		dateText += "d. " + p.DeathDate
	}

	nameLabel := widget.NewLabel(nameText)
	nameLabel.Wrapping = fyne.TextWrapOff
	if isRoot {
		nameLabel.TextStyle.Bold = true
	}

	dateLabel := widget.NewLabel(dateText)
	dateLabel.TextStyle.Italic = true
	dateLabel.Wrapping = fyne.TextWrapOff

	content := container.NewVBox(nameLabel, dateLabel)
	
	// Apply zoom to box size
	boxWidth := 200 * dv.zoomLevel
	boxHeight := 80 * dv.zoomLevel
	content.Resize(fyne.NewSize(boxWidth, boxHeight))

	// Get background color based on color mode
	bgColor := dv.getColorForPerson(p)
	
	// Create background rectangle
	bg := canvas.NewRectangle(bgColor)
	
	// Create a secondary button for collapse/expand
	var collapseBtn fyne.CanvasObject
	if generation < dv.maxGenerations {
		children := dv.getChildren(p.ID)
		if len(children) > 0 {
			collapseBtn = widget.NewButton("↕", func() {
				dv.collapsedBranches[p.ID] = !dv.collapsedBranches[p.ID]
				dv.refresh()
			})
		}
	}
	
	// Create base container with background and content
	var baseContainer fyne.CanvasObject
	if collapseBtn != nil {
		baseContainer = container.NewStack(bg, container.NewPadded(container.NewBorder(nil, collapseBtn, nil, nil, content)))
	} else {
		baseContainer = container.NewStack(bg, container.NewPadded(content))
	}
	
	// Wrap in TappableContainer for left-click (navigate) and right-click (context menu)
	tap := NewTappableContainer(
		baseContainer,
		func() {
			// Left-click: Navigate to person
			if dv.onNavigate != nil {
				dv.onNavigate(p.ID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Show context menu
			ShowPersonContextMenu(p, dv.store, dv.window, e.AbsolutePosition, dv.onNavigate, func() {
				dv.refresh()
				dv.Refresh()
			})
		},
	)
	
	boxed := tap
	boxed.Resize(fyne.NewSize(boxWidth, boxHeight))

	return boxed
}

// getColorForPerson returns the appropriate color based on the current color mode
func (dv *DescendantView) getColorForPerson(p *store.Person) color.Color {
	switch dv.colorMode {
	case ColorByGender:
		if p.Gender == "M" {
			return color.RGBA{R: 173, G: 216, B: 230, A: 255} // Light blue
		} else if p.Gender == "F" {
			return color.RGBA{R: 255, G: 182, B: 193, A: 255} // Light pink
		}
		return theme.BackgroundColor()
		
	case ColorByLivingStatus:
		if p.IsLiving {
			return color.RGBA{R: 144, G: 238, B: 144, A: 255} // Light green for living
		} else if p.DeathDate != "" {
			return color.RGBA{R: 192, G: 192, B: 192, A: 255} // Gray for deceased
		}
		return theme.BackgroundColor()
		
	case ColorByDataCompleteness:
		completeness := dv.calculateDataCompleteness(p)
		if completeness >= 80 {
			return color.RGBA{R: 144, G: 238, B: 144, A: 255} // Light green for complete
		} else if completeness >= 50 {
			return color.RGBA{R: 255, G: 255, B: 153, A: 255} // Light yellow for partial
		} else {
			return color.RGBA{R: 255, G: 182, B: 193, A: 255} // Light pink for incomplete
		}
		
	default:
		return theme.BackgroundColor()
	}
}

// calculateDataCompleteness returns a percentage of how complete the person's data is
func (dv *DescendantView) calculateDataCompleteness(p *store.Person) int {
	total := 0
	complete := 0
	
	// Core fields (weighted more)
	fields := []string{p.GivenName, p.Surname, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace}
	for _, field := range fields {
		total += 2
		if field != "" {
			complete += 2
		}
	}
	
	// Additional fields
	if p.Gender != "" {
		complete++
	}
	total++
	
	if p.Notes != "" {
		complete++
	}
	total++
	
	// Media
	if media, _ := dv.store.GetMediaForPerson(p.ID); len(media) > 0 {
		complete++
	}
	total++
	
	// Sources
	if count, _ := dv.store.CountCitationsForPerson(p.ID); count > 0 {
		complete++
	}
	total++
	
	if total == 0 {
		return 0
	}
	
	return (complete * 100) / total
}

// getChildren returns all children for a person
func (dv *DescendantView) getChildren(personID int64) []store.Person {
	children, _ := dv.store.GetRelatedPeople(personID, "child")
	return children
}

// showExportDialog shows export options
func (dv *DescendantView) showExportDialog() {
	chartContent := dv.buildDescendantChart()
	if dv.currentPerson == nil {
		dialog.ShowError(fmt.Errorf("No person selected"), dv.window)
		return
	}
	title := fmt.Sprintf("Descendant Chart - %s", formatPersonName(*dv.currentPerson))
	
	// Collect all people in the chart
	people := dv.collectAllDescendants(dv.currentPerson, 1)
	
	showChartExportDialogWithStore(dv.window, title, chartContent, people, dv.store)
}

// collectAllDescendants recursively collects all descendants for export
func (dv *DescendantView) collectAllDescendants(person *store.Person, generation int) []*store.Person {
	if person == nil || generation > dv.maxGenerations {
		return nil
	}
	
	people := []*store.Person{person}
	
	// Get children
	children, _ := dv.store.GetRelatedPeople(person.ID, "child")
	for i := range children {
		childPeople := dv.collectAllDescendants(&children[i], generation+1)
		people = append(people, childPeople...)
	}
	
	return people
}

// UpdateAllDescendantCharts updates all open descendant charts to show a new person
func UpdateAllDescendantCharts(s *store.Store, personID int64) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		return
	}
	
	for _, descView := range openDescendantCharts {
		descView.SetPerson(person)
		if descView.window != nil {
			descView.window.SetTitle(fmt.Sprintf("Descendant Chart - %s", formatPersonName(*person)))
		}
	}
}

// showDescendantChartDialog opens a new window with the descendant chart for a person
func showDescendantChartDialog(w fyne.Window, s *store.Store, personID int64, onNavigate func(int64)) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %v", err), w)
		return
	}

	// Create descendant chart window
	descWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Descendant Chart - %s", formatPersonName(*person)))
	descWindow.Resize(fyne.NewSize(1000, 700))

	// Create descendant view (declare first to avoid closure issue)
	var descView *DescendantView
	descView = NewDescendantView(s, descWindow, func(pid int64) {
		// Navigate in main window
		if onNavigate != nil {
			onNavigate(pid)
		}
		// Also update the descendant chart to show this person's descendants
		if newPerson, err := s.GetPersonByID(pid); err == nil {
			descView.SetPerson(newPerson)
			descWindow.SetTitle(fmt.Sprintf("Descendant Chart - %s", formatPersonName(*newPerson)))
		}
	})
	descView.SetPerson(person)
	
	// Track this descendant chart for dynamic updates
	openDescendantCharts = append(openDescendantCharts, descView)
	
	// Remove from list when window closes
	descWindow.SetOnClosed(func() {
		for i, dc := range openDescendantCharts {
			if dc == descView {
				openDescendantCharts = append(openDescendantCharts[:i], openDescendantCharts[i+1:]...)
				break
			}
		}
	})

	descWindow.SetContent(descView)
	descWindow.Show()
}
