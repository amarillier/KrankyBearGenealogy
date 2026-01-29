package ui

import (
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// FanChart displays ancestors in a circular/radial layout
type FanChart struct {
	widget.BaseWidget
	store          *store.Store
	currentPerson  *store.Person
	window         fyne.Window
	onNavigate     func(personID int64)
	maxGenerations int
	content        *fyne.Container
	enhancedMode   bool // Show spouses and children
	
	// Visual settings
	centerRadius   float32
	generationSize float32
	
	// Cached ancestor data
	ancestors map[int][]ancestorPosition
}

type ancestorPosition struct {
	person      *store.Person
	generation  int
	position    int // Position within generation
	startAngle  float64
	endAngle    float64
	innerRadius float32
	outerRadius float32
}

// NewFanChart creates a new fan chart widget
func NewFanChart(s *store.Store, w fyne.Window, onNavigate func(personID int64)) *FanChart {
	fc := &FanChart{
		store:          s,
		window:         w,
		onNavigate:     onNavigate,
		maxGenerations: 5,
		centerRadius:   100, // Distance from center to first ring
		generationSize: 120, // Distance between rings
		ancestors:      make(map[int][]ancestorPosition),
	}
	fc.ExtendBaseWidget(fc)
	return fc
}

// SetPerson updates the view to show a specific person's fan chart
func (fc *FanChart) SetPerson(p *store.Person) {
	fc.currentPerson = p
	fc.refresh()
	fc.Refresh()
}

// CreateRenderer implements the widget interface
func (fc *FanChart) CreateRenderer() fyne.WidgetRenderer {
	// Create control panel
	controlPanel := fc.createControlPanel()
	
	// Create chart container
	chartContainer := container.NewMax()
	fc.content = chartContainer
	
	// Build layout with controls at top
	layout := container.NewBorder(controlPanel, nil, nil, nil, chartContainer)
	
	fc.refresh()
	return widget.NewSimpleRenderer(layout)
}

// createControlPanel creates the control panel
func (fc *FanChart) createControlPanel() fyne.CanvasObject {
	// Generation selector
	genSelect := widget.NewSelect(
		[]string{"3 Generations", "4 Generations", "5 Generations", "6 Generations"},
		func(s string) {
			switch s {
			case "3 Generations":
				fc.maxGenerations = 3
			case "4 Generations":
				fc.maxGenerations = 4
			case "5 Generations":
				fc.maxGenerations = 5
			case "6 Generations":
				fc.maxGenerations = 6
			}
			fc.refresh()
		},
	)
	genSelect.SetSelected("5 Generations")
	
	// Enhanced mode checkbox
	enhancedCheck := widget.NewCheck("Enhanced (show spouses & children)", func(checked bool) {
		fc.enhancedMode = checked
		fc.refresh()
	})
	enhancedCheck.SetChecked(false)
	
	// Export button
	exportBtn := widget.NewButton("Export Chart", func() {
		chartContent := fc.buildFanChart()
		title := fmt.Sprintf("Fan Chart - %s", formatPersonName(*fc.currentPerson))
		
		// Collect all people in the chart with nil preservation for Ahnentafel indexing
		people := []*store.Person{fc.currentPerson}
		// Collect up to 4 generations for export (same as pedigree)
		maxExportGen := 4
		if fc.maxGenerations < maxExportGen {
			maxExportGen = fc.maxGenerations
		}
		for gen := 2; gen <= maxExportGen; gen++ {
			ancestors := fc.getAncestorsForGeneration(fc.currentPerson, gen, 1)
			people = append(people, ancestors...)
		}
		
		showChartExportDialog(fc.window, title, chartContent, people)
	})
	
	// Info button
	infoBtn := widget.NewButton("Legend", func() {
		fc.showLegend()
	})
	
	controlRow := container.NewHBox(
		widget.NewLabel("Generations:"),
		genSelect,
		widget.NewSeparator(),
		enhancedCheck,
		widget.NewSeparator(),
		infoBtn,
		exportBtn,
	)
	
	return container.NewVBox(controlRow, widget.NewSeparator())
}

// refresh rebuilds the fan chart
func (fc *FanChart) refresh() {
	if fc.content == nil {
		return
	}
	fc.content.Objects = nil
	
	if fc.currentPerson == nil {
		fc.content.Objects = []fyne.CanvasObject{widget.NewLabel("No person selected")}
		fc.content.Refresh()
		return
	}
	
	// Build the fan chart
	chart := fc.buildFanChart()
	scroll := container.NewScroll(chart)
	
	fc.content.Objects = []fyne.CanvasObject{scroll}
	fc.content.Refresh()
}

// buildFanChart creates the circular ancestor chart
func (fc *FanChart) buildFanChart() fyne.CanvasObject {
	// Calculate compact chart dimensions for laptop-friendly display
	adjustedGenSize := fc.generationSize
	
	// Scale for compact, laptop-friendly display (max radius ~300px = 600px canvas)
	switch fc.maxGenerations {
	case 3:
		adjustedGenSize = 90  // Spacious for few generations
	case 4:
		adjustedGenSize = 75
	case 5:
		adjustedGenSize = 55  // Compact but readable
	case 6:
		adjustedGenSize = 45  // Very compact for 6 generations
	}
	
	// Calculate total chart radius based on adjusted spacing
	chartRadius := fc.centerRadius + float32(fc.maxGenerations-1)*adjustedGenSize + 40
	
	// Cap at 300px radius for laptop-friendly 700x700 window (600px canvas + margins)
	maxRadius := float32(300)
	if chartRadius > maxRadius {
		chartRadius = maxRadius
		adjustedGenSize = (chartRadius - fc.centerRadius - 40) / float32(fc.maxGenerations-1)
	}
	
	// Create title
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Fan Chart for %s (%d Generations)", 
			formatPersonName(*fc.currentPerson), fc.maxGenerations),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	
	// Temporarily adjust generation size for building
	oldGenSize := fc.generationSize
	fc.generationSize = adjustedGenSize
	
	// Build ancestor data structure
	fc.buildAncestorData()
	
	// Restore original generation size
	fc.generationSize = oldGenSize
	
	// Create the chart image (canvas will be 2*chartRadius in each dimension)
	chartImage := fc.drawFanChartCanvas(chartRadius)
	
	// Wrap in scroll container for resizability
	scroll := container.NewScroll(chartImage)
	
	return container.NewBorder(title, nil, nil, nil, scroll)
}

// buildAncestorData constructs the ancestor position data
func (fc *FanChart) buildAncestorData() {
	fc.ancestors = make(map[int][]ancestorPosition)
	
	// Generation 1 is just the current person (handled separately as center)
	
	// Build each generation in rings around the center
	for gen := 2; gen <= fc.maxGenerations; gen++ {
		ancestors := fc.getAncestorsForGeneration(fc.currentPerson, gen, 1)
		positions := []ancestorPosition{}
		
		numInGen := 1 << (gen - 1) // 2^(gen-1) - number of ancestors at this generation
		anglePerPerson := 360.0 / float64(numInGen)
		
		// Calculate ring radius - spread out more as generations increase
		ringRadius := fc.centerRadius + float32(gen-2)*fc.generationSize
		innerRadius := ringRadius - fc.generationSize/4
		outerRadius := ringRadius + fc.generationSize/4
		
		for i, ancestor := range ancestors {
			// Distribute evenly around the full 360 degrees
			// Start at top (0 degrees = 12 o'clock) and go clockwise
			// First half = father's line (0-180), second half = mother's line (180-360)
			centerAngle := float64(i) * anglePerPerson
			
			positions = append(positions, ancestorPosition{
				person:      ancestor,
				generation:  gen,
				position:    i,
				startAngle:  centerAngle - anglePerPerson/2,
				endAngle:    centerAngle + anglePerPerson/2,
				innerRadius: innerRadius,
				outerRadius: outerRadius,
			})
		}
		
		fc.ancestors[gen] = positions
	}
}

// getAncestorsForGeneration gets ancestors at a specific generation
func (fc *FanChart) getAncestorsForGeneration(person *store.Person, targetGen, currentGen int) []*store.Person {
	if person == nil {
		numAtGen := 1 << (targetGen - currentGen)
		return make([]*store.Person, numAtGen)
	}
	
	if currentGen == targetGen {
		return []*store.Person{person}
	}
	
	// Get parents
	parents := fc.getParents(person.ID)
	var father, mother *store.Person
	for i := range parents {
		if parents[i].Gender == "M" {
			father = &parents[i]
		} else {
			mother = &parents[i]
		}
	}
	
	// Recurse
	fatherAncestors := fc.getAncestorsForGeneration(father, targetGen, currentGen+1)
	motherAncestors := fc.getAncestorsForGeneration(mother, targetGen, currentGen+1)
	
	return append(fatherAncestors, motherAncestors...)
}

// drawFanChartCanvas creates a complete fan chart with simple boxes positioned in a circle
func (fc *FanChart) drawFanChartCanvas(chartSize float32) fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	centerX := chartSize
	centerY := chartSize
	
	// Draw center person box
	centerBox := fc.createPersonBox(fc.currentPerson, centerX, centerY, true)
	objects = append(objects, centerBox...)
	
	// If enhanced mode, add spouses and children
	if fc.enhancedMode {
		// Get spouses (to the right)
		spouses, _ := fc.store.GetRelatedPeople(fc.currentPerson.ID, "spouse")
		spouseY := centerY
		for i, spouse := range spouses {
			// Position spouses to the right of center person
			spouseBox := fc.createEnhancedBox(&spouse, centerX+170, spouseY+float32(i*80), "spouse")
			objects = append(objects, spouseBox...)
		}
		
		// Get children (below)
		children, _ := fc.store.GetRelatedPeople(fc.currentPerson.ID, "child")
		childX := centerX - float32(len(children)-1)*50 // Center the children horizontally
		for i, child := range children {
			childBox := fc.createEnhancedBox(&child, childX+float32(i*100), centerY+110, "child")
			objects = append(objects, childBox...)
		}
	}
	
	// Draw all ancestors as positioned boxes
	for gen := 2; gen <= fc.maxGenerations; gen++ {
		positions := fc.ancestors[gen]
		for _, pos := range positions {
			if pos.person != nil {
				personBox := fc.createAncestorBox(pos, centerX, centerY)
				objects = append(objects, personBox...)
			}
		}
	}
	
	// Create container with all objects
	cont := container.NewWithoutLayout(objects...)
	cont.Resize(fyne.NewSize(chartSize*2, chartSize*2))
	
	return cont
}

// createPersonBox creates a simple box for the center person
func (fc *FanChart) createPersonBox(person *store.Person, centerX, centerY float32, isCenter bool) []fyne.CanvasObject {
	// Get gender color
	bgColor := fc.getGenderColor(person)
	
	// Create rounded rectangle
	boxSize := float32(120)
	if isCenter {
		boxSize = 140
	}
	
	bg := canvas.NewRectangle(bgColor)
	bg.StrokeColor = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	bg.StrokeWidth = 2
	bg.CornerRadius = 8
	bg.Resize(fyne.NewSize(boxSize, boxSize/2))
	bg.Move(fyne.NewPos(centerX-boxSize/2, centerY-boxSize/4))
	
	// Add name
	nameText := fc.getPersonShortName(person)
	label := widget.NewLabel(nameText)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle.Bold = true
	label.Wrapping = fyne.TextWrapWord
	label.Resize(fyne.NewSize(boxSize-10, boxSize/2-10))
	label.Move(fyne.NewPos(centerX-boxSize/2+5, centerY-boxSize/4+5))
	
	// Create base stack with background and label
	baseStack := container.NewStack(bg, label)
	
	// Wrap in TappableContainer for left-click and right-click
	personID := person.ID
	personPtr := person // Capture for closure
	tappable := NewTappableContainer(
		baseStack,
		func() {
			// Left-click: Navigate
			if fc.onNavigate != nil {
				fc.onNavigate(personID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Context menu
			ShowPersonContextMenu(personPtr, fc.store, fc.window, e.AbsolutePosition, fc.onNavigate, func() {
				fc.refresh()
			})
		},
	)
	tappable.Resize(fyne.NewSize(boxSize, boxSize/2))
	tappable.Move(fyne.NewPos(centerX-boxSize/2, centerY-boxSize/4))
	
	return []fyne.CanvasObject{tappable}
}

// createAncestorBox creates a simple box for an ancestor positioned in a ring
func (fc *FanChart) createAncestorBox(pos ancestorPosition, centerX, centerY float32) []fyne.CanvasObject {
	// Calculate position in the ring
	midAngle := (pos.startAngle + pos.endAngle) / 2.0
	radius := pos.innerRadius + (pos.outerRadius-pos.innerRadius)/2.0
	angleRad := midAngle * math.Pi / 180.0
	
	x := centerX + float32(math.Cos(angleRad))*radius
	y := centerY + float32(math.Sin(angleRad))*radius
	
	// Get gender color
	bgColor := fc.getGenderColor(pos.person)
	
	// Create small box
	boxWidth := float32(90)
	boxHeight := float32(45)
	
	bg := canvas.NewRectangle(bgColor)
	bg.StrokeColor = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	bg.StrokeWidth = 1.5
	bg.CornerRadius = 5
	bg.Resize(fyne.NewSize(boxWidth, boxHeight))
	bg.Move(fyne.NewPos(x-boxWidth/2, y-boxHeight/2))
	
	// Add name
	nameText := fc.getPersonShortName(pos.person)
	label := widget.NewLabel(nameText)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle.Bold = true
	label.Wrapping = fyne.TextWrapWord
	label.Resize(fyne.NewSize(boxWidth-6, boxHeight-6))
	label.Move(fyne.NewPos(x-boxWidth/2+3, y-boxHeight/2+3))
	
	// Create base stack with background and label
	baseStack := container.NewStack(bg, label)
	
	// Wrap in TappableContainer for left-click and right-click
	personID := pos.person.ID
	personPtr := pos.person // Capture for closure
	tappable := NewTappableContainer(
		baseStack,
		func() {
			// Left-click: Navigate
			if fc.onNavigate != nil {
				fc.onNavigate(personID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Context menu
			ShowPersonContextMenu(personPtr, fc.store, fc.window, e.AbsolutePosition, fc.onNavigate, func() {
				fc.refresh()
			})
		},
	)
	tappable.Resize(fyne.NewSize(boxWidth, boxHeight))
	tappable.Move(fyne.NewPos(x-boxWidth/2, y-boxHeight/2))
	
	return []fyne.CanvasObject{tappable}
}

// createEnhancedBox creates a box for spouses or children in enhanced mode
func (fc *FanChart) createEnhancedBox(person *store.Person, x, y float32, relType string) []fyne.CanvasObject {
	// Get gender color
	bgColor := fc.getGenderColor(person)
	
	// Slightly smaller boxes for spouses/children
	boxWidth := float32(85)
	boxHeight := float32(42)
	
	bg := canvas.NewRectangle(bgColor)
	bg.StrokeColor = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	bg.StrokeWidth = 1.5
	bg.CornerRadius = 5
	bg.Resize(fyne.NewSize(boxWidth, boxHeight))
	bg.Move(fyne.NewPos(x-boxWidth/2, y-boxHeight/2))
	
	// Add name
	nameText := fc.getPersonShortName(person)
	label := widget.NewLabel(nameText)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle.Bold = true
	label.Wrapping = fyne.TextWrapWord
	label.Resize(fyne.NewSize(boxWidth-6, boxHeight-6))
	label.Move(fyne.NewPos(x-boxWidth/2+3, y-boxHeight/2+3))
	
	// Create base stack with background and label
	baseStack := container.NewStack(bg, label)
	
	// Wrap in TappableContainer for left-click and right-click
	personID := person.ID
	personPtr := person // Capture for closure
	tappable := NewTappableContainer(
		baseStack,
		func() {
			// Left-click: Navigate
			if fc.onNavigate != nil {
				fc.onNavigate(personID)
			}
		},
		func(e *fyne.PointEvent) {
			// Right-click: Context menu
			ShowPersonContextMenu(personPtr, fc.store, fc.window, e.AbsolutePosition, fc.onNavigate, func() {
				fc.refresh()
			})
		},
	)
	tappable.Resize(fyne.NewSize(boxWidth, boxHeight))
	tappable.Move(fyne.NewPos(x-boxWidth/2, y-boxHeight/2))
	
	return []fyne.CanvasObject{tappable}
}

// getGenderColor returns pink for females, blue for males, gray for unknown
func (fc *FanChart) getGenderColor(person *store.Person) color.Color {
	if person == nil {
		return &color.RGBA{R: 220, G: 220, B: 220, A: 255}
	}
	
	switch person.Gender {
	case "M":
		return &color.RGBA{R: 173, G: 216, B: 230, A: 255} // Light blue
	case "F":
		return &color.RGBA{R: 255, G: 182, B: 193, A: 255} // Light pink
	default:
		return &color.RGBA{R: 240, G: 240, B: 240, A: 255} // Light gray
	}
}

// getPersonShortName returns a shortened name for display
func (fc *FanChart) getPersonShortName(p *store.Person) string {
	if p == nil {
		return "Unknown"
	}
	
	name := ""
	if p.PreferredName != "" {
		name = p.PreferredName
	} else if p.GivenName != "" {
		name = p.GivenName
	}
	
	if p.Surname != "" {
		if name != "" {
			name += " "
		}
		name += p.Surname
	}
	
	if name == "" {
		name = "Unknown"
	}
	
	// Truncate if too long
	if len(name) > 15 {
		name = name[:12] + "..."
	}
	
	return name
}

// getParents returns the parents of a person
func (fc *FanChart) getParents(personID int64) []store.Person {
	parents, _ := fc.store.GetRelatedPeople(personID, "parent")
	return parents
}

// showLegend shows a legend explaining the fan chart
func (fc *FanChart) showLegend() {
	legendText := "Fan Chart Legend:\n\n" +
		"• Center box: Current person\n" +
		"• Each ring outward: One generation back (ancestors)\n" +
		"• Blue boxes: Males\n" +
		"• Pink boxes: Females\n" +
		"• Gray boxes: Unknown gender\n" +
		"• Click any box to navigate to that person\n" +
		"• Ancestors spread in full circle (360°)\n\n" +
		"Enhanced Mode (optional):\n" +
		"• Spouses: Shown to the right of center person\n" +
		"• Children: Shown below center person\n\n" +
		fmt.Sprintf("Currently showing %d generations", fc.maxGenerations)
	
	if fc.enhancedMode {
		legendText += " (Enhanced mode enabled)"
	}
	
	dialog.ShowInformation("Fan Chart Legend", legendText, fc.window)
}

// Global list of open fan charts for synchronization
var openFanCharts []*FanChart

// showFanChartDialog opens a dialog window with the fan chart
func showFanChartDialog(w fyne.Window, s *store.Store, personID int64, onNavigate func(int64)) {
	// Get the person
	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Cannot show fan chart: %w", err), w)
		return
	}
	
	// Create fan chart window - compact, laptop-friendly size
	fanWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Fan Chart - %s", formatPersonName(*person)))
	fanWindow.Resize(fyne.NewSize(700, 700))
	
	// Create fan chart widget (declare as var to use in closure)
	var fanChart *FanChart
	fanChart = NewFanChart(s, fanWindow, func(pid int64) {
		onNavigate(pid)
		// Update fan chart to new person
		if newPerson, err := s.GetPersonByID(pid); err == nil {
			fanChart.SetPerson(newPerson)
			fanWindow.SetTitle(fmt.Sprintf("Fan Chart - %s", formatPersonName(*newPerson)))
		}
	})
	
	// Set initial person
	fanChart.SetPerson(person)
	
	// Add to global list for synchronization
	openFanCharts = append(openFanCharts, fanChart)
	
	// Remove from list when window closes
	fanWindow.SetOnClosed(func() {
		for i, fc := range openFanCharts {
			if fc == fanChart {
				openFanCharts = append(openFanCharts[:i], openFanCharts[i+1:]...)
				break
			}
		}
	})
	
	// Set content and show
	fanWindow.SetContent(fanChart)
	fanWindow.Show()
}

// UpdateAllFanCharts updates all open fan charts to show a specific person
func UpdateAllFanCharts(s *store.Store, personID int64) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		return
	}
	
	for _, fanChart := range openFanCharts {
		fanChart.SetPerson(person)
		if fanChart.window != nil {
			fanChart.window.SetTitle(fmt.Sprintf("Fan Chart - %s", formatPersonName(*person)))
		}
	}
}
