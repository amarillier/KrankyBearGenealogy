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

// ColorMode defines how to color-code the pedigree chart
type ColorMode int

const (
	ColorNone ColorMode = iota
	ColorByGender
	ColorByLivingStatus
	ColorByDataCompleteness
)

// PedigreeView displays an ancestor chart in PAF style.
type PedigreeView struct {
	widget.BaseWidget
	store            *store.Store
	currentPerson    *store.Person
	window           fyne.Window
	onNavigate       func(personID int64)
	onEdit           func(personID int64)
	content          *fyne.Container
	
	// Enhanced features
	maxGenerations   int
	colorMode        ColorMode
	zoomLevel        float32
	collapsedBranches map[int64]bool // person ID -> is collapsed
	
	// UI controls
	genSelect        *widget.Select
	colorSelect      *widget.Select
	zoomSlider       *widget.Slider
}

// NewPedigreeView creates a new pedigree chart widget.
func NewPedigreeView(s *store.Store, w fyne.Window, onNavigate func(personID int64), onEdit func(personID int64)) *PedigreeView {
	pv := &PedigreeView{
		store:             s,
		window:            w,
		onNavigate:        onNavigate,
		onEdit:            onEdit,
		maxGenerations:    4,
		colorMode:         ColorNone,
		zoomLevel:         1.0,
		collapsedBranches: make(map[int64]bool),
	}
	pv.ExtendBaseWidget(pv)
	return pv
}

// SetPerson updates the view to show a specific person's pedigree.
func (pv *PedigreeView) SetPerson(p *store.Person) {
	pv.currentPerson = p
	pv.refresh()
	pv.Refresh() // Force widget refresh
}

// CreateRenderer implements the widget interface.
func (pv *PedigreeView) CreateRenderer() fyne.WidgetRenderer {
	// Create control panel
	controlPanel := pv.createControlPanel()
	
	// Create chart container
	chartContainer := container.NewMax()
	
	// Store reference for refresh
	pv.content = chartContainer
	
	// Build layout with controls at top
	layout := container.NewBorder(controlPanel, nil, nil, nil, chartContainer)
	
	pv.refresh()
	return widget.NewSimpleRenderer(layout)
}

// createControlPanel creates the control panel with generation, color, and zoom controls
func (pv *PedigreeView) createControlPanel() fyne.CanvasObject {
	// Generation selector
	pv.genSelect = widget.NewSelect([]string{"4 Generations", "5 Generations", "6 Generations", "7 Generations", "8 Generations"}, func(s string) {
		switch s {
		case "4 Generations":
			pv.maxGenerations = 4
		case "5 Generations":
			pv.maxGenerations = 5
		case "6 Generations":
			pv.maxGenerations = 6
		case "7 Generations":
			pv.maxGenerations = 7
		case "8 Generations":
			pv.maxGenerations = 8
		}
		pv.refresh()
	})
	pv.genSelect.SetSelected("4 Generations")
	
	// Color mode selector
	pv.colorSelect = widget.NewSelect([]string{"No Color", "By Gender", "By Living Status", "By Data Completeness"}, func(s string) {
		switch s {
		case "No Color":
			pv.colorMode = ColorNone
		case "By Gender":
			pv.colorMode = ColorByGender
		case "By Living Status":
			pv.colorMode = ColorByLivingStatus
		case "By Data Completeness":
			pv.colorMode = ColorByDataCompleteness
		}
		pv.refresh()
	})
	pv.colorSelect.SetSelected("No Color")
	
	// Zoom slider
	pv.zoomSlider = widget.NewSlider(0.5, 2.0)
	pv.zoomSlider.Value = 1.0
	pv.zoomSlider.Step = 0.1
	pv.zoomSlider.OnChanged = func(v float64) {
		pv.zoomLevel = float32(v)
		pv.refresh()
	}
	
	// Zoom label
	zoomLabel := widget.NewLabel("Zoom: 100%")
	pv.zoomSlider.OnChanged = func(v float64) {
		pv.zoomLevel = float32(v)
		zoomLabel.SetText(fmt.Sprintf("Zoom: %.0f%%", v*100))
		pv.refresh()
	}
	
	// Expand all button
	expandAllBtn := widget.NewButton("Expand All", func() {
		pv.collapsedBranches = make(map[int64]bool)
		pv.refresh()
	})
	
	// Export button
	exportBtn := widget.NewButton("Export Chart", func() {
		pv.showExportDialog()
	})
	
	// Layout controls in a horizontal bar
	controlRow := container.NewHBox(
		widget.NewLabel("Generations:"),
		pv.genSelect,
		widget.NewSeparator(),
		widget.NewLabel("Colors:"),
		pv.colorSelect,
		widget.NewSeparator(),
		zoomLabel,
		pv.zoomSlider,
		widget.NewSeparator(),
		expandAllBtn,
		exportBtn,
	)
	
	return container.NewVBox(controlRow, widget.NewSeparator())
}

func (pv *PedigreeView) refresh() {
	if pv.content == nil {
		return
	}
	pv.content.Objects = nil

	if pv.currentPerson == nil {
		pv.content.Objects = []fyne.CanvasObject{widget.NewLabel("No person selected")}
		pv.content.Refresh()
		return
	}

	// Build pedigree chart showing 4 generations
	chart := pv.buildPedigreeChart()
	scroll := container.NewScroll(chart)
	
	// Replace content with scroll container
	pv.content.Objects = []fyne.CanvasObject{scroll}
	pv.content.Refresh()
}

// buildPedigreeChart creates the pedigree chart layout with dynamic generations.
func (pv *PedigreeView) buildPedigreeChart() fyne.CanvasObject {
	person := pv.currentPerson
	
	// Build columns for each generation
	columns := make([]fyne.CanvasObject, pv.maxGenerations)
	
	// Generation 1: Current person
	gen1Label := pv.getGenerationLabel(1)
	personBox := pv.makePersonBox(person, true, 1)
	columns[0] = container.NewVBox(gen1Label, widget.NewLabel(""), personBox)
	
	// Build remaining generations recursively
	for gen := 2; gen <= pv.maxGenerations; gen++ {
		columns[gen-1] = pv.buildGenerationColumn(gen)
	}
	
	// Apply zoom to chart
	chart := container.NewHBox(columns...)
	if pv.zoomLevel != 1.0 {
		// Wrap in a scaled container
		chart = container.NewPadded(chart)
	}
	
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Pedigree Chart for %s (%d Generations)", formatPersonName(*person), pv.maxGenerations),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	return container.NewBorder(title, nil, nil, nil, chart)
}

// buildGenerationColumn builds a column for a specific generation
func (pv *PedigreeView) buildGenerationColumn(generation int) fyne.CanvasObject {
	label := pv.getGenerationLabel(generation)
	
	// Build ancestor list for this generation
	ancestors := pv.getAncestorsAtGeneration(pv.currentPerson, generation, 1)
	
	boxes := []fyne.CanvasObject{label}
	
	// Add spacing after label
	if generation == 2 {
		boxes = append(boxes, widget.NewLabel("Father:"))
	}
	
	for i, ancestor := range ancestors {
		if ancestor != nil && !pv.collapsedBranches[ancestor.ID] {
			boxes = append(boxes, pv.makePersonBox(ancestor, false, generation))
		} else {
			boxes = append(boxes, pv.makeEmptyBox())
		}
		
		// Add spacing between father and mother in generation 2
		if generation == 2 && i == 0 {
			boxes = append(boxes, widget.NewLabel(""))
			boxes = append(boxes, widget.NewLabel("Mother:"))
		}
	}
	
	return container.NewVBox(boxes...)
}

// getAncestorsAtGeneration recursively gets ancestors at a specific generation
func (pv *PedigreeView) getAncestorsAtGeneration(person *store.Person, targetGen, currentGen int) []*store.Person {
	if person == nil {
		numAtGen := 1 << (targetGen - currentGen) // 2^(targetGen - currentGen)
		return make([]*store.Person, numAtGen)
	}
	
	if currentGen == targetGen {
		return []*store.Person{person}
	}
	
	// Check if this branch is collapsed
	if pv.collapsedBranches[person.ID] && currentGen > 1 {
		numAtGen := 1 << (targetGen - currentGen)
		return make([]*store.Person, numAtGen)
	}
	
	// Get parents
	parents := pv.getParents(person.ID)
	var father, mother *store.Person
	for i := range parents {
		if parents[i].Gender == "M" {
			father = &parents[i]
		} else {
			mother = &parents[i]
		}
	}
	
	// Recurse into father and mother branches
	fatherAncestors := pv.getAncestorsAtGeneration(father, targetGen, currentGen+1)
	motherAncestors := pv.getAncestorsAtGeneration(mother, targetGen, currentGen+1)
	
	// Combine (father's line first, then mother's line)
	return append(fatherAncestors, motherAncestors...)
}

// getGenerationLabel returns the ordinal label for a generation
func (pv *PedigreeView) getGenerationLabel(gen int) *widget.Label {
	suffixes := []string{"st", "nd", "rd", "th", "th", "th", "th", "th"}
	suffix := "th"
	if gen <= 3 {
		suffix = suffixes[gen-1]
	}
	return widget.NewLabel(fmt.Sprintf("%d%s", gen, suffix))
}

// makePersonBox creates a clickable box for a person with color coding.
func (pv *PedigreeView) makePersonBox(p *store.Person, isCurrent bool, generation int) fyne.CanvasObject {
	nameText := formatPersonName(*p)
	
	// Check if person has media and add indicator
	if media, err := pv.store.GetMediaForPerson(p.ID); err == nil && len(media) > 0 {
		nameText = "📷 " + nameText
	}
	
	// Add bookmark indicator if bookmarked
	if p.Bookmarked {
		nameText = "★ " + nameText
	}
	
	// Add todo indicator if person has pending todos
	if count, _ := pv.store.CountPendingTodosForPerson(p.ID); count > 0 {
		nameText = "📝 " + nameText
	}
	
	// Add source indicator if person has citations
	if count, _ := pv.store.CountCitationsForPerson(p.ID); count > 0 {
		nameText = "📚 " + nameText
	}

	// Add research log indicator if person has research logs
	if count, _ := pv.store.CountResearchLogsForPerson(p.ID); count > 0 {
		nameText = "🔍 " + nameText
	}
	
	// Add collapse indicator if person has parents and is not generation 1
	if generation < pv.maxGenerations {
		parents := pv.getParents(p.ID)
		if len(parents) > 0 {
			if pv.collapsedBranches[p.ID] {
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
	if isCurrent {
		nameLabel.TextStyle.Bold = true
	}

	dateLabel := widget.NewLabel(dateText)
	dateLabel.TextStyle.Italic = true
	dateLabel.Wrapping = fyne.TextWrapOff

	content := container.NewVBox(nameLabel, dateLabel)
	
	// Apply zoom to box size
	boxWidth := 200 * pv.zoomLevel
	boxHeight := 80 * pv.zoomLevel
	content.Resize(fyne.NewSize(boxWidth, boxHeight))

	// Get background color based on color mode
	bgColor := pv.getColorForPerson(p)
	
	// Create background rectangle
	bg := canvas.NewRectangle(bgColor)
	
	// Create a secondary button for collapse/expand
	var collapseBtn fyne.CanvasObject
	if generation < pv.maxGenerations {
		parents := pv.getParents(p.ID)
		if len(parents) > 0 {
			collapseBtn = widget.NewButton("↕", func() {
				pv.collapsedBranches[p.ID] = !pv.collapsedBranches[p.ID]
				pv.refresh()
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
			if pv.onNavigate != nil {
				pv.onNavigate(p.ID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Show context menu
			ShowPersonContextMenu(p, pv.store, pv.window, e.AbsolutePosition, pv.onNavigate, func() {
				pv.refresh()
				pv.Refresh()
			})
		},
	)
	
	boxed := tap
	boxed.Resize(fyne.NewSize(boxWidth, boxHeight))

	return boxed
}

// getColorForPerson returns the background color for a person based on color mode
func (pv *PedigreeView) getColorForPerson(p *store.Person) color.Color {
	switch pv.colorMode {
	case ColorByGender:
		if p.Gender == "M" {
			return &color.RGBA{R: 173, G: 216, B: 230, A: 255} // Light blue
		} else if p.Gender == "F" {
			return &color.RGBA{R: 255, G: 182, B: 193, A: 255} // Light pink
		}
		return &color.RGBA{R: 230, G: 230, B: 230, A: 255} // Light gray
		
	case ColorByLivingStatus:
		if p.IsLiving {
			return &color.RGBA{R: 144, G: 238, B: 144, A: 255} // Light green
		}
		return &color.RGBA{R: 211, G: 211, B: 211, A: 255} // Light gray
		
	case ColorByDataCompleteness:
		completeness := pv.calculateDataCompleteness(p)
		if completeness >= 0.8 {
			return &color.RGBA{R: 144, G: 238, B: 144, A: 255} // Green (80%+)
		} else if completeness >= 0.5 {
			return &color.RGBA{R: 255, G: 255, B: 153, A: 255} // Yellow (50-79%)
		}
		return &color.RGBA{R: 255, G: 182, B: 193, A: 255} // Pink (<50%)
		
	default:
		return theme.ButtonColor()
	}
}

// calculateDataCompleteness calculates how complete a person's data is (0.0 to 1.0)
func (pv *PedigreeView) calculateDataCompleteness(p *store.Person) float64 {
	total := 0.0
	complete := 0.0
	
	// Name fields (3 points total)
	total += 3
	if p.GivenName != "" {
		complete += 1
	}
	if p.Surname != "" {
		complete += 1
	}
	if p.PreferredName != "" {
		complete += 1
	}
	
	// Birth info (2 points)
	total += 2
	if p.BirthDate != "" {
		complete += 1
	}
	if p.BirthPlace != "" {
		complete += 1
	}
	
	// Death info (2 points if deceased)
	if !p.IsLiving {
		total += 2
		if p.DeathDate != "" {
			complete += 1
		}
		if p.DeathPlace != "" {
			complete += 1
		}
	}
	
	// Gender (1 point)
	total += 1
	if p.Gender != "" {
		complete += 1
	}
	
	if total == 0 {
		return 0
	}
	return complete / total
}

// makeEmptyBox creates an empty placeholder box.
func (pv *PedigreeView) makeEmptyBox() fyne.CanvasObject {
	label := widget.NewLabel("Unknown")
	label.Alignment = fyne.TextAlignCenter
	bg := canvas.NewRectangle(theme.DisabledColor())
	box := container.NewMax(bg, container.NewPadded(label))
	box.Resize(fyne.NewSize(200, 100))
	return box
}

// makePersonBoxIfExists creates a box if person exists, otherwise empty box.
func (pv *PedigreeView) makePersonBoxIfExists(p *store.Person, generation int) fyne.CanvasObject {
	if p == nil {
		return pv.makeEmptyBox()
	}
	return pv.makePersonBox(p, false, generation)
}

// showExportDialog shows options for exporting the pedigree chart
func (pv *PedigreeView) showExportDialog() {
	dialog.ShowInformation("Export Chart",
		"Chart export functionality coming soon!\n\n"+
			"Future options will include:\n"+
			"• Export to PDF\n"+
			"• Export to PNG image\n"+
			"• Print directly\n"+
			"• Share via email",
		pv.window)
}

// getParents returns the parents of a person.
func (pv *PedigreeView) getParents(personID int64) []store.Person {
	parents, _ := pv.store.GetRelatedPeople(personID, "parent")
	return parents
}

// tappableContainer wraps a container to handle single/double taps.
type tappableContainer struct {
	widget.BaseWidget
	content      fyne.CanvasObject
	onTap        func(doubleTap bool)
	lastTapTime  int64
	tapThreshold int64 // milliseconds
}

func newTappableContainer(content fyne.CanvasObject, onTap func(doubleTap bool)) *tappableContainer {
	t := &tappableContainer{
		content:      content,
		onTap:        onTap,
		tapThreshold: 300, // 300ms for double-tap
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *tappableContainer) Tapped(e *fyne.PointEvent) {
	// Simple single tap for now - double tap detection is complex in Fyne
	// User can use the "Go to Family View" button for navigation
	if t.onTap != nil {
		t.onTap(false) // Always single tap
	}
}

func (t *tappableContainer) DoubleTapped(e *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap(true)
	}
}
