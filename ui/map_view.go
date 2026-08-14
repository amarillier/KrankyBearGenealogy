package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/jung-kurt/gofpdf"

	"genealogy/store"
)

// MapView displays an interactive map with person locations and life events.
type MapView struct {
	widget.BaseWidget
	store          *store.Store
	window         fyne.Window
	onNavigate     func(personID int64)
	currentPerson  *store.Person
	tileDownloader *TileDownloader

	// Map state
	centerLat float64
	centerLon float64
	zoomLevel int
	mapCanvas *fyne.Container

	// View mode and filters (Phase 5)
	viewMode          string // "person", "all", "descendants", "ancestors"
	filterSurname     string
	filterStartYear   int
	filterEndYear     int
	filterLiving      string // "all", "living", "deceased"
	colorByGeneration bool

	// Marker filtering
	showBirths    bool
	showDeaths    bool
	showMarriages bool
	showEvents    bool // Show generic Life Events markers
	showPaths     bool // Show migration paths connecting life events
	markers       []*MapMarker

	// Timeline slider ("as of year" time-lapse filter)
	timelineYear    int
	timelineMinYear int
	timelineMaxYear int

	// Measure tool (click two points to see the distance between them)
	measureMode   bool
	measurePointA *LatLng
	measurePointB *LatLng

	// Cached projection basis from the most recent render, so a click
	// (which happens outside renderTileGridWithMarkers) can invert the same
	// lat/lng<->pixel projection the render used.
	lastGridMinX   int
	lastGridMinY   int
	lastGridStartX float32
	lastGridStartY float32

	// UI components
	content            *fyne.Container
	controlPanel       *fyne.Container
	zoomInBtn          *widget.Button
	zoomOutBtn         *widget.Button
	zoomLabel          *widget.Label
	statusLabel        *widget.Label
	loadingLabel       *widget.Label
	markerStatsLabel   *widget.Label
	viewModeSelect     *widget.Select
	surnameEntry       *widget.Entry
	startYearEntry     *widget.Entry
	endYearEntry       *widget.Entry
	livingSelect       *widget.Select
	generationCheck    *widget.Check
	timelineSlider     *widget.Slider
	timelineLabel      *widget.Label
	measureBtn         *widget.Button
	measureStatusLabel *widget.Label
}

// NewMapView creates a new map view window.
func NewMapView(s *store.Store, w fyne.Window, onNavigate func(personID int64)) *MapView {
	mv := &MapView{
		store:             s,
		window:            w,
		onNavigate:        onNavigate,
		tileDownloader:    NewTileDownloader(),
		centerLat:         40.0, // Default center (approximate US center)
		centerLon:         -95.0,
		zoomLevel:         4, // Country-level view
		showBirths:        true,
		showDeaths:        true,
		showMarriages:     true,
		showEvents:        true,
		showPaths:         true, // Show migration paths by default
		viewMode:          "person", // Default to person-specific view
		filterLiving:      "all",    // Show all by default
		filterStartYear:   0,        // No date filter by default
		filterEndYear:     0,
		colorByGeneration: false, // Off by default
	}
	mv.ExtendBaseWidget(mv)
	return mv
}

// CreateRenderer implements fyne.Widget.
func (mv *MapView) CreateRenderer() fyne.WidgetRenderer {
	mv.buildUI()
	return widget.NewSimpleRenderer(mv.content)
}

// buildUI constructs the map view interface.
func (mv *MapView) buildUI() {
	// Map canvas - use NewWithoutLayout for absolute positioning
	mv.loadingLabel = widget.NewLabel("Loading map tiles...")
	bg := canvas.NewRectangle(color.RGBA{R: 200, G: 220, B: 240, A: 255})
	bg.Resize(fyne.NewSize(900, 600))  // Wider to match window
	mv.mapCanvas = container.NewWithoutLayout(bg, container.NewCenter(mv.loadingLabel))
	mv.mapCanvas.Resize(fyne.NewSize(900, 600))  // Set explicit size

	// Control panel
	mv.buildControlPanel()

	// Status bar
	mv.statusLabel = widget.NewLabel("Map View - OpenStreetMap")
	
	// Update status if we have a current person
	if mv.currentPerson != nil {
		if mv.currentPerson.BirthPlace != "" {
			mv.statusLabel.SetText(fmt.Sprintf("Viewing: %s (Born: %s)", 
				formatPersonName(*mv.currentPerson), mv.currentPerson.BirthPlace))
		} else {
			mv.statusLabel.SetText(fmt.Sprintf("Viewing: %s", formatPersonName(*mv.currentPerson)))
		}
	}
	
	statusBar := container.NewBorder(nil, nil, nil, nil, mv.statusLabel)

	// Wrap map canvas in a scroll container to enforce bounds/clipping
	// This prevents map content from rendering outside its area
	mapScroll := container.NewScroll(mv.mapCanvas)
	mapScroll.SetMinSize(fyne.NewSize(900, 600))
	
	// Main layout - control panel clearly separated from map
	mv.content = container.NewBorder(
		mv.controlPanel,  // Top (controls)
		statusBar,        // Bottom (status)
		nil,              // Left
		nil,              // Right
		mapScroll,        // Center (map in scroll container for clipping)
	)
	
	// Load initial tiles in background
	go mv.loadMapTiles()
}

// buildControlPanel creates the map control toolbar.
func (mv *MapView) buildControlPanel() {
	// Zoom controls
	mv.zoomInBtn = widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		mv.zoomIn()
	})
	mv.zoomInBtn.Importance = widget.LowImportance

	mv.zoomOutBtn = widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), func() {
		mv.zoomOut()
	})
	mv.zoomOutBtn.Importance = widget.LowImportance

	mv.zoomLabel = widget.NewLabel(fmt.Sprintf("Zoom: %d", mv.zoomLevel))

	reloadBtn := widget.NewButton("🔄 Reload", func() {
		mv.markers = nil // Clear marker cache
		mv.refreshMap()
	})
	reloadBtn.Importance = widget.LowImportance

	jumpToEventBtn := widget.NewButton("📍 Jump to Event...", func() {
		mv.showEventListDialog()
	})
	jumpToEventBtn.Importance = widget.LowImportance

	// Pan buttons for map navigation
	panUpBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		mv.panMap(0, -1) // Pan up
	})
	panUpBtn.Importance = widget.LowImportance
	
	panDownBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		mv.panMap(0, 1) // Pan down
	})
	panDownBtn.Importance = widget.LowImportance
	
	panLeftBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		mv.panMap(-1, 0) // Pan left
	})
	panLeftBtn.Importance = widget.LowImportance
	
	panRightBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		mv.panMap(1, 0) // Pan right
	})
	panRightBtn.Importance = widget.LowImportance
	
	// Arrange pan buttons in a cross pattern
	panControls := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), panUpBtn, layout.NewSpacer()),
		container.NewHBox(panLeftBtn, panDownBtn, panRightBtn),
	)

	zoomControls := container.NewHBox(
		mv.zoomInBtn,
		mv.zoomOutBtn,
		mv.zoomLabel,
		widget.NewSeparator(),
		panControls,
		layout.NewSpacer(),
		reloadBtn,
		jumpToEventBtn,
	)

	// Marker filter controls with re-center buttons
	showBirthsCheck := widget.NewCheck("🟢 Births", func(checked bool) {
		mv.showBirths = checked
		mv.refreshMap()
	})
	showBirthsCheck.SetChecked(mv.showBirths)
	
	centerBirthBtn := widget.NewButton("Center", func() {
		mv.centerOnEventType("birth")
	})
	centerBirthBtn.Importance = widget.LowImportance

	showDeathsCheck := widget.NewCheck("⚫ Deaths", func(checked bool) {
		mv.showDeaths = checked
		mv.refreshMap()
	})
	showDeathsCheck.SetChecked(mv.showDeaths)
	
	centerDeathBtn := widget.NewButton("Center", func() {
		mv.centerOnEventType("death")
	})
	centerDeathBtn.Importance = widget.LowImportance

	showMarriagesCheck := widget.NewCheck("💒 Marriages", func(checked bool) {
		mv.showMarriages = checked
		mv.refreshMap()
	})
	showMarriagesCheck.SetChecked(mv.showMarriages)
	
	centerMarriageBtn := widget.NewButton("Center", func() {
		mv.centerOnEventType("marriage")
	})
	centerMarriageBtn.Importance = widget.LowImportance

	showPathsCheck := widget.NewCheck("🔀 Migration Paths", func(checked bool) {
		mv.showPaths = checked
		mv.refreshMap()
	})
	showPathsCheck.SetChecked(mv.showPaths)

	showEventsCheck := widget.NewCheck("📅 Life Events", func(checked bool) {
		mv.showEvents = checked
		mv.refreshMap()
	})
	showEventsCheck.SetChecked(mv.showEvents)

	// Measure tool: toggle click-to-measure mode on the map.
	mv.measureStatusLabel = widget.NewLabel("")
	mv.measureBtn = widget.NewButton("📏 Measure", func() {
		mv.measureMode = !mv.measureMode
		mv.measurePointA = nil
		mv.measurePointB = nil
		if mv.measureMode {
			mv.measureBtn.Importance = widget.HighImportance
			mv.measureStatusLabel.SetText("Click a point on the map to start measuring...")
		} else {
			mv.measureBtn.Importance = widget.LowImportance
			mv.measureStatusLabel.SetText("")
		}
		mv.measureBtn.Refresh()
		mv.refreshMap()
	})
	mv.measureBtn.Importance = widget.LowImportance

	filterControls := container.NewHBox(
		widget.NewLabel("Show:"),
		showBirthsCheck,
		centerBirthBtn,
		showDeathsCheck,
		centerDeathBtn,
		showMarriagesCheck,
		centerMarriageBtn,
		showPathsCheck,
		showEventsCheck,
		widget.NewSeparator(),
		mv.measureBtn,
		mv.measureStatusLabel,
	)

	// View mode selector (Phase 5) - wider for better visibility
	mv.viewModeSelect = widget.NewSelect([]string{"Person", "All People", "Descendants", "Ancestors"}, func(value string) {
		switch value {
		case "Person":
			mv.viewMode = "person"
		case "All People":
			mv.viewMode = "all"
		case "Descendants":
			mv.viewMode = "descendants"
		case "Ancestors":
			mv.viewMode = "ancestors"
		}
		// Force reload markers with new view mode
		mv.markers = nil
		mv.refreshMap()
	})
	mv.viewModeSelect.SetSelected("Person")

	mv.generationCheck = widget.NewCheck("Color by Generation", func(checked bool) {
		mv.colorByGeneration = checked
		mv.refreshMap()
	})

	// Filter controls (Phase 5) - use form layout for proper sizing
	mv.surnameEntry = widget.NewEntry()
	mv.surnameEntry.SetPlaceHolder("Surname...")
	mv.surnameEntry.OnChanged = func(value string) {
		mv.filterSurname = value
	}
	mv.surnameEntry.OnSubmitted = func(value string) {
		mv.filterSurname = value
		mv.refreshMap()
	}

	mv.startYearEntry = widget.NewEntry()
	mv.startYearEntry.SetPlaceHolder("From year")
	mv.startYearEntry.OnSubmitted = func(value string) {
		if value == "" {
			mv.filterStartYear = 0
		} else {
			fmt.Sscanf(value, "%d", &mv.filterStartYear)
		}
		mv.refreshMap()
	}

	mv.endYearEntry = widget.NewEntry()
	mv.endYearEntry.SetPlaceHolder("To year")
	mv.endYearEntry.OnSubmitted = func(value string) {
		if value == "" {
			mv.filterEndYear = 0
		} else {
			fmt.Sscanf(value, "%d", &mv.filterEndYear)
		}
		mv.refreshMap()
	}

	mv.livingSelect = widget.NewSelect([]string{"All", "Living", "Deceased"}, func(value string) {
		mv.filterLiving = strings.ToLower(value)
		mv.refreshMap()
	})
	mv.livingSelect.SetSelected("All")

	// Create filter button to apply filters explicitly
	applyFiltersBtn := widget.NewButton("Apply Filters", func() {
		// Explicitly read current values from entry fields before refreshing
		mv.filterSurname = mv.surnameEntry.Text
		if mv.startYearEntry.Text == "" {
			mv.filterStartYear = 0
		} else {
			fmt.Sscanf(mv.startYearEntry.Text, "%d", &mv.filterStartYear)
		}
		if mv.endYearEntry.Text == "" {
			mv.filterEndYear = 0
		} else {
			fmt.Sscanf(mv.endYearEntry.Text, "%d", &mv.filterEndYear)
		}
		
		// Force markers to be reloaded with new filter values
		mv.markers = nil
		mv.refreshMap()
	})
	applyFiltersBtn.Importance = widget.HighImportance

	// Marker statistics label
	mv.markerStatsLabel = widget.NewLabel("")
	mv.markerStatsLabel.TextStyle = fyne.TextStyle{Italic: true}

	// View mode controls in a grid for proper spacing
	viewModeRow := container.NewGridWithColumns(4,
		widget.NewLabel("View:"),
		mv.viewModeSelect,
		mv.generationCheck,
		mv.markerStatsLabel,
	)

	// Filter controls in a grid for proper spacing and wider fields
	filterRow := container.NewGridWithColumns(8,
		widget.NewLabel("Surname:"),
		mv.surnameEntry,
		widget.NewLabel("Years:"),
		mv.startYearEntry,
		widget.NewLabel("-"),
		mv.endYearEntry,
		widget.NewLabel("Status:"),
		mv.livingSelect,
	)
	
	// Filter action row
	filterActionRow := container.NewHBox(
		applyFiltersBtn,
		layout.NewSpacer(),
	)

	// Timeline slider: time-lapse the map to only show events up to a
	// given year. Range is populated once markers load (updateTimelineRange).
	mv.timelineLabel = widget.NewLabel("As of: —")
	mv.timelineSlider = widget.NewSlider(1900, 2026)
	mv.timelineSlider.Step = 1
	mv.timelineSlider.OnChanged = func(v float64) {
		mv.timelineLabel.SetText(fmt.Sprintf("As of: %d", int(v)))
	}
	mv.timelineSlider.OnChangeEnded = func(v float64) {
		mv.timelineYear = int(v)
		mv.refreshMap()
	}

	timelineRow := container.NewBorder(
		nil, nil,
		widget.NewLabel("Timeline:"),
		mv.timelineLabel,
		mv.timelineSlider,
	)

	// Export button
	exportBtn := widget.NewButtonWithIcon("Export...", theme.DocumentSaveIcon(), func() {
		mv.showExportDialog()
	})
	exportBtn.Importance = widget.LowImportance

	// Help button
	helpBtn := widget.NewButtonWithIcon("Help", theme.HelpIcon(), func() {
		mv.showHelp()
	})
	helpBtn.Importance = widget.LowImportance

	// Close button - close the window
	closeBtn := widget.NewButtonWithIcon("Close", theme.CancelIcon(), func() {
		if mv.window != nil {
			mv.window.Close()
		}
	})
	closeBtn.Importance = widget.LowImportance

	// Assemble control panel with clear separation and better layout
	// Row 1: Zoom and marker controls with buttons at the end
	row1 := container.NewBorder(
		nil, nil,
		container.NewHBox(zoomControls, widget.NewSeparator(), filterControls),
		container.NewHBox(exportBtn, helpBtn, closeBtn),
	)
	
	// Row 2: View mode selection
	row2 := container.NewBorder(
		nil, nil,
		viewModeRow,
		nil,
	)
	
	// Row 3: Filters
	row3 := container.NewBorder(
		nil, nil,
		filterRow,
		filterActionRow,
	)

	// Assemble all rows with background color for visibility
	mv.controlPanel = container.NewVBox(
		row1,
		widget.NewSeparator(),
		row2,
		widget.NewSeparator(),
		row3,
		widget.NewSeparator(),
		timelineRow,
		widget.NewSeparator(), // Bottom separator to clearly separate from map
	)
}

// zoomIn increases the zoom level.
func (mv *MapView) zoomIn() {
	if mv.zoomLevel < 18 { // Max zoom
		mv.zoomLevel++
		mv.zoomLabel.SetText(fmt.Sprintf("Zoom: %d", mv.zoomLevel))
		mv.refreshMap()
	}
}

// zoomOut decreases the zoom level.
func (mv *MapView) zoomOut() {
	if mv.zoomLevel > 1 { // Min zoom
		mv.zoomLevel--
		mv.zoomLabel.SetText(fmt.Sprintf("Zoom: %d", mv.zoomLevel))
		mv.refreshMap()
	}
}

// panMap shifts the map center by a fraction of the viewport.
// dx and dy are direction multipliers (-1, 0, or 1).
func (mv *MapView) panMap(dx, dy int) {
	// Calculate pan distance based on zoom level
	// At each zoom level, moving by 1/4 of the viewport feels natural
	n := math.Pow(2, float64(mv.zoomLevel))
	degreesPerTile := 360.0 / n
	
	// Pan by approximately 1/2 of visible area
	panFactor := 0.5
	
	// Update longitude (wraps around at ±180)
	mv.centerLon += float64(dx) * degreesPerTile * panFactor
	for mv.centerLon > 180 {
		mv.centerLon -= 360
	}
	for mv.centerLon < -180 {
		mv.centerLon += 360
	}
	
	// Update latitude (clamped to ±85 for Mercator projection)
	mv.centerLat += float64(dy) * degreesPerTile * panFactor
	if mv.centerLat > 85 {
		mv.centerLat = 85
	}
	if mv.centerLat < -85 {
		mv.centerLat = -85
	}
	
	// Refresh map
	mv.refreshMap()
}

// refreshMap redraws the map with current settings.
func (mv *MapView) refreshMap() {
	// Update status label if it exists (may not exist during initial UI building)
	if mv.statusLabel != nil {
		mv.statusLabel.SetText(fmt.Sprintf("Center: %.4f, %.4f | Zoom: %d", mv.centerLat, mv.centerLon, mv.zoomLevel))
	}
	// Load tiles in background
	go mv.loadMapTiles()
}

// centerOnEventType re-centers the map on a specific event type marker for the current person.
func (mv *MapView) centerOnEventType(eventType string) {
	// Load markers if not loaded yet
	if mv.markers == nil {
		mv.loadMarkers()
	}
	
	// Convert string to MarkerType
	var targetType MarkerType
	var eventName string
	switch eventType {
	case "birth":
		targetType = MarkerBirth
		eventName = "birth"
	case "death":
		targetType = MarkerDeath
		eventName = "death"
	case "marriage":
		targetType = MarkerMarriage
		eventName = "marriage"
	default:
		return
	}
	
	// Find the marker for this event type
	var targetMarker *MapMarker
	for _, marker := range mv.markers {
		if marker.Type == targetType {
			targetMarker = marker
			break
		}
	}
	
	if targetMarker == nil {
		// Show message that this event has no location
		dialog.ShowInformation("No Location", 
			fmt.Sprintf("No location information available for %s event.", eventName),
			mv.window)
		return
	}
	
	// Update center to this marker's location
	mv.centerLat = targetMarker.Latitude
	mv.centerLon = targetMarker.Longitude

	// Refresh the map to show the new center
	mv.refreshMap()
}

// centerOnMarker recenters (and, if zoomed out too far to make a single
// event obvious, zooms in on) the given marker's location.
func (mv *MapView) centerOnMarker(marker *MapMarker) {
	mv.centerLat = marker.Latitude
	mv.centerLon = marker.Longitude
	if mv.zoomLevel < 8 {
		mv.zoomLevel = 8
		if mv.zoomLabel != nil {
			mv.zoomLabel.SetText(fmt.Sprintf("Zoom: %d", mv.zoomLevel))
		}
	}
	mv.refreshMap()
}

// eventMarkerLabel formats a marker for display in the "Jump to Event" list.
func eventMarkerLabel(m *MapMarker) string {
	style := GetMarkerStyle(m.Type)

	typeLabel := m.EventLabel
	switch m.Type {
	case MarkerBirth:
		typeLabel = "Birth"
	case MarkerDeath:
		typeLabel = "Death"
	case MarkerMarriage:
		typeLabel = "Marriage"
		if m.SpouseName != "" {
			typeLabel = fmt.Sprintf("Marriage to %s", m.SpouseName)
		}
	case MarkerCurrentAddress:
		typeLabel = "Current Address"
	}

	date := m.EventDate
	if date == "" {
		date = "Unknown date"
	}
	return fmt.Sprintf("%s  %s — %s — %s", style.Symbol, typeLabel, date, m.Place)
}

// showEventListDialog lists every geocoded event for the current person
// (birth, death, marriages, and life events) and recenters/zooms the map on
// whichever one is clicked - makes it obvious the feature exists and lets
// the user jump straight to an event even if its marker is hard to spot at
// the current zoom level.
func (mv *MapView) showEventListDialog() {
	if mv.currentPerson == nil {
		dialog.ShowInformation("No Person Selected", "Select a person to see their events.", mv.window)
		return
	}

	markers, err := GetMarkersForPerson(mv.store, mv.currentPerson.ID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load events: %w", err), mv.window)
		return
	}
	if len(markers) == 0 {
		dialog.ShowInformation("No Events", "No geocoded events found for this person yet.", mv.window)
		return
	}

	sort.Slice(markers, func(i, j int) bool {
		return parseDateForSort(markers[i].EventDate).Before(parseDateForSort(markers[j].EventDate))
	})

	var eventDialog dialog.Dialog
	list := container.NewVBox()
	for _, marker := range markers {
		m := marker
		btn := widget.NewButton(eventMarkerLabel(m), func() {
			eventDialog.Hide()
			mv.centerOnMarker(m)
		})
		btn.Alignment = widget.ButtonAlignLeading
		list.Add(btn)
	}

	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(420, 400))

	eventDialog = dialog.NewCustom("Jump to Event", "Close", scroll, mv.window)
	eventDialog.Resize(fyne.NewSize(460, 500))
	eventDialog.Show()
}

// loadMapTiles downloads and renders map tiles for the current view.
// This runs in a goroutine, so all UI updates must use fyne.Do().
func (mv *MapView) loadMapTiles() {
	// Safety check
	if mv.zoomLevel < 1 || mv.zoomLevel > 18 {
		return
	}
	
	// Update loading label on UI thread
	if mv.loadingLabel != nil {
		fyne.Do(func() {
			mv.loadingLabel.SetText("Loading map tiles...")
		})
	}
	
	// Calculate viewport size (approximation based on window size)
	viewportWidth := 900
	viewportHeight := 600
	
	// Get visible tiles
	tiles := GetVisibleTiles(mv.centerLat, mv.centerLon, mv.zoomLevel, viewportWidth, viewportHeight)
	
	// Safety check
	if len(tiles) == 0 {
		if mv.loadingLabel != nil {
			fyne.Do(func() {
				mv.loadingLabel.SetText("No tiles to load")
			})
		}
		return
	}
	
	// Download tiles (max 3 concurrent to respect OSM usage policy)
	// This is the heavy I/O work that happens in background
	tileImages, err := mv.tileDownloader.DownloadTiles(tiles, 3)
	
	// Check for network connectivity issues
	if err != nil {
		fyne.Do(func() {
			if mv.loadingLabel != nil {
				mv.loadingLabel.SetText("No internet connection")
			}
			// Create dialog with callback to close map window when dismissed
			d := dialog.NewInformation("Map Unavailable",
				"Map tiles could not be loaded.\n\n"+
				"This feature requires an active internet connection to download map data from OpenStreetMap.\n\n"+
				"Please check your network connection and try again.",
				mv.window)
			d.SetOnClosed(func() {
				// Close the map window when user dismisses the dialog
				mv.window.Close()
			})
			d.Show()
		})
		return
	}
	
	// Update UI on main thread
	fyne.Do(func() {
		// Build tile grid with markers
		mv.renderTileGridWithMarkers(tiles, tileImages)
		
		if mv.loadingLabel != nil {
			markerCount := 0
			if mv.markers != nil {
				markerCount = len(mv.markers)
			}
			mv.loadingLabel.SetText(fmt.Sprintf("Loaded %d tiles, %d markers", len(tileImages), markerCount))
		}
	})
}

// renderTileGridWithMarkers arranges tiles and markers in a grid layout.
func (mv *MapView) renderTileGridWithMarkers(tiles []MapTile, tileImages map[string]*canvas.Image) {
	if len(tileImages) == 0 {
		// No tiles loaded
		bg := canvas.NewRectangle(color.RGBA{R: 200, G: 220, B: 240, A: 255})
		bg.Resize(fyne.NewSize(800, 600))
		mv.mapCanvas.Objects = []fyne.CanvasObject{
			bg,
			container.NewCenter(widget.NewLabel("Failed to load map tiles")),
		}
		mv.mapCanvas.Refresh()
		return
	}
	
	const tileSize = 256
	minX, _, minY, _ := calculateTileBounds(tiles)
	
	// Calculate marker's position in tile coordinate system
	centerTileCoord := LatLngToTile(mv.centerLat, mv.centerLon, mv.zoomLevel)
	centerPixelOffset := latLngToTilePixelOffset(mv.centerLat, mv.centerLon, mv.zoomLevel)
	
	// Marker position relative to tile grid origin (minX, minY)
	markerRelX := float32((centerTileCoord.X - minX) * tileSize) + centerPixelOffset.X
	markerRelY := float32((centerTileCoord.Y - minY) * tileSize) + centerPixelOffset.Y
	
	// We want the marker centered in 900x600 viewport at (450, 300)
	// Position the tile grid so marker ends up there
	gridStartX := 450 - markerRelX
	gridStartY := 300 - markerRelY

	// Cache the projection basis so a click (which happens outside this
	// function, after this render) can be converted back to lat/lng.
	mv.lastGridMinX = minX
	mv.lastGridMinY = minY
	mv.lastGridStartX = gridStartX
	mv.lastGridStartY = gridStartY

	// Canvas is fixed at 900x600 to match window
	canvasWidth := float32(900)
	canvasHeight := float32(600)

	// Create grid container
	gridObjects := make([]fyne.CanvasObject, 0)

	// Add sized background, wrapped so clicks on empty map area can be
	// captured (used by the measure tool).
	bg := canvas.NewRectangle(color.RGBA{R: 200, G: 220, B: 240, A: 255})
	bg.Resize(fyne.NewSize(canvasWidth, canvasHeight))
	clickCatcher := newMapClickCatcher(bg, mv.handleMapClick)
	// The catcher is a widget.BaseWidget, not a plain canvas object - its own
	// hit-region defaults to zero size until explicitly resized/moved (the
	// inner rectangle's own Resize above doesn't propagate to the wrapper).
	clickCatcher.Resize(fyne.NewSize(canvasWidth, canvasHeight))
	clickCatcher.Move(fyne.NewPos(0, 0))
	gridObjects = append(gridObjects, clickCatcher)
	
	// Add tiles
	for _, tile := range tiles {
		key := fmt.Sprintf("%d-%d-%d", tile.Zoom, tile.X, tile.Y)
		img, ok := tileImages[key]
		if !ok {
			continue
		}
		
		// Calculate position relative to tile grid origin
		gridX := tile.X - minX
		gridY := tile.Y - minY
		
		// Position the tile starting from gridStart (0,0)
		tileX := gridStartX + float32(gridX*tileSize)
		tileY := gridStartY + float32(gridY*tileSize)
		img.Resize(fyne.NewSize(tileSize, tileSize))
		img.Move(fyne.NewPos(tileX, tileY))
		gridObjects = append(gridObjects, img)
	}
	
	// Load markers if not loaded yet
	if mv.markers == nil {
		mv.loadMarkers()
	}
	
	// Filter markers based on user preferences
	filteredMarkers := mv.filterMarkers()
	
	// Draw migration paths if enabled (before markers so paths appear underneath)
	if mv.showPaths {
		pathObjects := mv.drawMigrationPaths(filteredMarkers, minX, minY, gridStartX, gridStartY, tileSize)
		gridObjects = append(gridObjects, pathObjects...)
	}
	
	// Add markers using the same coordinate system as tiles
	for _, marker := range filteredMarkers {
		// Capture marker in closure
		m := marker
		
		// Convert lat/lng to tile coordinates
		tileCoord := LatLngToTile(m.Latitude, m.Longitude, mv.zoomLevel)
		pixelOffset := latLngToTilePixelOffset(m.Latitude, m.Longitude, mv.zoomLevel)
		
		// Calculate position: tiles start at gridStart, so marker is offset from that
		markerX := gridStartX + float32((tileCoord.X - minX) * tileSize) + pixelOffset.X - 10  // -10 to center 20x20 marker
		markerY := gridStartY + float32((tileCoord.Y - minY) * tileSize) + pixelOffset.Y - 10
		
		// Create marker widget with optional generation coloring (Phase 5).
		// While the measure tool is active, tapping a marker registers a
		// measure point at its exact coordinates instead of opening its
		// popup - otherwise there'd be no way to measure to/from a pin
		// without clicking just off it.
		markerWidget := CreateMarkerWidget(m, func() {
			if mv.measureMode {
				mv.registerMeasurePoint(m.Latitude, m.Longitude)
				return
			}
			mv.showMarkerPopup(m)
		}, mv.colorByGeneration)
		
		// Position marker
		markerWidget.Resize(fyne.NewSize(20, 20))
		markerWidget.Move(fyne.NewPos(markerX, markerY))
		
		gridObjects = append(gridObjects, markerWidget)
	}

	// Draw the measure tool overlay (points + distance line), on top of
	// markers so it stays visible.
	measureObjects := mv.drawMeasureOverlay(minX, minY, gridStartX, gridStartY)
	gridObjects = append(gridObjects, measureObjects...)

	// Set canvas size to fit all tiles
	mv.mapCanvas.Resize(fyne.NewSize(canvasWidth, canvasHeight))
	
	// Update canvas objects
	mv.mapCanvas.Objects = gridObjects
	mv.mapCanvas.Refresh()
}

// loadMarkers loads markers based on view mode and filters (Phase 5).
func (mv *MapView) loadMarkers() {
	var markers []*MapMarker
	var err error

	// Load markers based on view mode
	switch mv.viewMode {
	case "person":
		// Show only current person's markers
		if mv.currentPerson != nil {
			markers, err = GetMarkersForPerson(mv.store, mv.currentPerson.ID)
		}
	case "all":
		// Show all markers
		markers, err = GetAllMarkers(mv.store)
	case "descendants":
		// Show descendants of current person
		if mv.currentPerson != nil {
			markers, err = GetMarkersForDescendants(mv.store, mv.currentPerson.ID)
		}
	case "ancestors":
		// Show ancestors of current person
		if mv.currentPerson != nil {
			markers, err = GetMarkersForAncestors(mv.store, mv.currentPerson.ID)
		}
	default:
		// Fallback to person mode
		if mv.currentPerson != nil {
			markers, err = GetMarkersForPerson(mv.store, mv.currentPerson.ID)
		}
	}

	if err != nil {
		return
	}

	// Apply filters (Phase 5)
	filtered := make([]*MapMarker, 0)
	for _, marker := range markers {
		// Apply surname filter
		if mv.filterSurname != "" {
			personNameLower := strings.ToLower(marker.PersonName)
			surnameLower := strings.ToLower(mv.filterSurname)
			if !strings.Contains(personNameLower, surnameLower) {
				continue
			}
		}

		// Apply date range filter
		if mv.filterStartYear > 0 || mv.filterEndYear > 0 {
			// Extract year from event date (handles formats like "25 Jun 2015", "2015", etc.)
			eventYear := extractYearFromDate(marker.EventDate)
			
			// Only apply filter if we successfully parsed a year (> 100 to avoid parsing day/month as year)
			if eventYear > 100 {
				if mv.filterStartYear > 0 && eventYear < mv.filterStartYear {
					continue
				}
				if mv.filterEndYear > 0 && eventYear > mv.filterEndYear {
					continue
				}
			}
		}

		// Apply living status filter
		if mv.filterLiving != "all" {
			person, err := mv.store.GetPersonByID(marker.PersonID)
			if err != nil {
				continue
			}
			
			isLiving := person.DeathDate == ""
			if mv.filterLiving == "living" && !isLiving {
				continue
			}
			if mv.filterLiving == "deceased" && isLiving {
				continue
			}
		}

		filtered = append(filtered, marker)
	}

	// Calculate generations if color-coding is enabled (Phase 5)
	if mv.colorByGeneration && mv.currentPerson != nil {
		CalculateGenerations(mv.store, filtered, mv.currentPerson.ID)
	}

	mv.markers = filtered

	// Update the timeline slider's range to span the currently loaded
	// markers, defaulting to "show everything" (the latest year).
	mv.updateTimelineRange()

	// Update marker statistics
	mv.updateMarkerStats()
}

// updateTimelineRange recomputes the timeline slider's min/max years from
// mv.markers and keeps mv.timelineYear defaulted to "show everything" unless
// the user has already narrowed it within the new range.
func (mv *MapView) updateTimelineRange() {
	minYear, maxYear := 0, 0
	for _, m := range mv.markers {
		year := extractYearFromDate(m.EventDate)
		if year <= 100 {
			continue
		}
		if minYear == 0 || year < minYear {
			minYear = year
		}
		if year > maxYear {
			maxYear = year
		}
	}
	if maxYear == 0 {
		// No dated markers loaded - fall back to a sensible default range.
		minYear, maxYear = 1900, 2026
	}

	mv.timelineMinYear = minYear
	mv.timelineMaxYear = maxYear
	if mv.timelineYear == 0 || mv.timelineYear > maxYear || mv.timelineYear < minYear {
		mv.timelineYear = maxYear
	}

	if mv.timelineSlider != nil {
		mv.timelineSlider.Min = float64(minYear)
		mv.timelineSlider.Max = float64(maxYear)
		mv.timelineSlider.Value = float64(mv.timelineYear)
		mv.timelineSlider.Refresh()
	}
	if mv.timelineLabel != nil {
		mv.timelineLabel.SetText(fmt.Sprintf("As of: %d", mv.timelineYear))
	}
}

// updateMarkerStats counts and displays marker statistics.
func (mv *MapView) updateMarkerStats() {
	birthCount := 0
	deathCount := 0
	marriageCount := 0
	eventCount := 0

	for _, marker := range mv.markers {
		switch marker.Type {
		case MarkerBirth:
			birthCount++
		case MarkerDeath:
			deathCount++
		case MarkerMarriage:
			marriageCount++
		case MarkerEvent:
			eventCount++
		}
	}

	// Update label on UI thread
	if mv.markerStatsLabel != nil {
		statsText := fmt.Sprintf("Births: %d | Deaths: %d | Marriages: %d | Events: %d", birthCount, deathCount, marriageCount, eventCount)
		fyne.Do(func() {
			mv.markerStatsLabel.SetText(statsText)
		})
	}
}

// filterMarkers filters markers based on user preferences.
func (mv *MapView) filterMarkers() []*MapMarker {
	filtered := []*MapMarker{}
	
	for _, marker := range mv.markers {
		include := false
		switch marker.Type {
		case MarkerBirth:
			include = mv.showBirths
		case MarkerDeath:
			include = mv.showDeaths
		case MarkerMarriage:
			include = mv.showMarriages
		case MarkerCurrentAddress:
			include = mv.showBirths // Show with births (same green color)
		case MarkerEvent:
			include = mv.showEvents
		}
		
		// Timeline slider: only show events on or before the selected year.
		// Markers with an unparseable date are always shown (tolerant, same
		// as the date-range filter in loadMarkers).
		if include && mv.timelineYear > 0 {
			year := extractYearFromDate(marker.EventDate)
			if year > 100 && year > mv.timelineYear {
				include = false
			}
		}

		if include {
			filtered = append(filtered, marker)
		}
	}

	return filtered
}

// Note: extractYearFromDate is defined in advanced_search.go

// latLngToTilePixelOffset calculates the pixel offset within a tile for a given lat/lng.
// Returns values in range [0, 256) for both X and Y.
func latLngToTilePixelOffset(lat, lng float64, zoom int) fyne.Position {
	const tileSize = 256
	n := math.Pow(2.0, float64(zoom))
	
	// Calculate tile coordinates (as floats)
	tileX := (lng + 180.0) / 360.0 * n
	latRad := lat * math.Pi / 180.0
	tileY := (1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n
	
	// Get fractional part (position within tile)
	fracX := tileX - math.Floor(tileX)
	fracY := tileY - math.Floor(tileY)
	
	// Convert to pixel offset within tile
	pixelX := fracX * tileSize
	pixelY := fracY * tileSize
	
	return fyne.NewPos(float32(pixelX), float32(pixelY))
}

// showMarkerPopup shows details about a marker when clicked.
func (mv *MapView) showMarkerPopup(marker *MapMarker) {
	var message string
	
	// Calculate age at event if possible
	var ageText string
	if marker.Type != MarkerBirth && marker.EventDate != "" {
		if person, err := mv.store.GetPersonByID(marker.PersonID); err == nil && person.BirthDate != "" {
			if age := calculateAgeAtEvent(person.BirthDate, marker.EventDate); age >= 0 {
				ageText = fmt.Sprintf("\nAge: %d years", age)
			}
		}
	}
	
	switch marker.Type {
	case MarkerBirth:
		message = fmt.Sprintf("Birth of %s\n\nDate: %s\nPlace: %s",
			marker.PersonName, marker.EventDate, marker.Place)
	case MarkerDeath:
		message = fmt.Sprintf("Death of %s\n\nDate: %s\nPlace: %s%s",
			marker.PersonName, marker.EventDate, marker.Place, ageText)
	case MarkerMarriage:
		message = fmt.Sprintf("Marriage\n\n%s\n& %s\n\nDate: %s\nPlace: %s%s",
			marker.PersonName, marker.SpouseName, marker.EventDate, marker.Place, ageText)
	case MarkerCurrentAddress:
		message = fmt.Sprintf("Current Address\n\n%s\n\nLocation: %s\n\nStatus: Living",
			marker.PersonName, marker.Place)
	case MarkerEvent:
		dateText := marker.EventDate
		if marker.EventDateEnd != "" {
			dateText = fmt.Sprintf("%s – %s", marker.EventDate, marker.EventDateEnd)
		}
		message = fmt.Sprintf("%s\n\n%s\n\nDate: %s\nPlace: %s",
			marker.EventLabel, marker.PersonName, dateText, marker.Place)
	}
	
	// Add navigation button
	message += "\n\nClick 'View Person' to navigate to this person's record"
	
	dialog := dialog.NewConfirm("Life Event", message, func(navigate bool) {
		if navigate {
			mv.onNavigate(marker.PersonID)
			// Update window title
			if person, err := mv.store.GetPersonByID(marker.PersonID); err == nil {
				mv.window.SetTitle(fmt.Sprintf("Map View - %s", formatPersonName(*person)))
				mv.SetPersonFocus(person)
			}
		}
	}, mv.window)
	
	dialog.SetDismissText("Close")
	dialog.SetConfirmText("View Person")
	dialog.Show()
}

// calculateTileBounds finds the min/max X/Y from a list of tiles.
func calculateTileBounds(tiles []MapTile) (minX, maxX, minY, maxY int) {
	if len(tiles) == 0 {
		return 0, 0, 0, 0
	}
	
	minX, maxX = tiles[0].X, tiles[0].X
	minY, maxY = tiles[0].Y, tiles[0].Y
	
	for _, tile := range tiles[1:] {
		if tile.X < minX {
			minX = tile.X
		}
		if tile.X > maxX {
			maxX = tile.X
		}
		if tile.Y < minY {
			minY = tile.Y
		}
		if tile.Y > maxY {
			maxY = tile.Y
		}
	}
	
	return minX, maxX, minY, maxY
}

// SetCenter updates the map center coordinates.
func (mv *MapView) SetCenter(lat, lon float64) {
	mv.centerLat = lat
	mv.centerLon = lon
	mv.refreshMap()
}

// showHelp displays map view help dialog.
func (mv *MapView) showHelp() {
	helpText := `Map View Help

🗺️ Map Controls:
  • Zoom In/Out: Use + and - buttons
  • Pan: Use the arrow buttons
  • Markers: Click to view person details

📍 Marker Types:
  • 🟢 Green: Birth locations / current address (living)
  • ⚫ Gray: Death locations
  • 💒 Pink: Marriage locations
  • 🔀 Migration Paths: Line from birth to death/current address

🔍 Filters:
  • Toggle checkboxes to show/hide event types
  • View: Person / All People / Descendants / Ancestors
  • Surname, year range, and living-status filters

⏳ Timeline Slider:
  • Drag to a year to show only events on or before that year -
    a time-lapse of the family's geographic footprint
  • Try it: open Map View → View: Descendants, focus on
    Elya Yelnats (from the demo database), keep Migration Paths
    checked, then drag the Timeline slider forward from 1850.
    You'll see his birth in Riga, Latvia; his 1875 marriage in
    Boston; his move to and death in New York; then his
    descendants' migration line stretch west to Dallas and Austin.

📏 Measure Tool:
  • Click "📏 Measure", then click two points on the map
  • Shows the distance between them in km and miles
  • Click a third point to start a new measurement

📍 Jump to Event:
  • Click "📍 Jump to Event..." for a chronological list of every
    geocoded event for the current person - birth, death, every
    marriage, and every Life Event
  • Click any entry to recenter (and zoom in on) that location -
    handy when several events are close together or far apart and
    hard to spot at the current zoom level

💡 Tips:
  • Right-click any person → "View on Map"
  • Map uses OpenStreetMap tiles (free, no API key)`

	dialog.ShowInformation("Map View Help", helpText, mv.window)
}

// ShowMapWindow opens the map view in a new window.
func ShowMapWindow(s *store.Store, parentWindow fyne.Window, onNavigate func(personID int64)) {
	mapView := NewMapView(s, parentWindow, onNavigate)

	// Create new window with room for control panel
	mapWindow := fyne.CurrentApp().NewWindow("Map View - KrankyBear Genealogy")
	mapWindow.Resize(fyne.NewSize(950, 800))  // Taller for control panel
	mapWindow.CenterOnScreen()

	// Set close button action
	if mapView.controlPanel != nil {
		// Find and update close button
		closeBtn := widget.NewButtonWithIcon("Close", theme.CancelIcon(), func() {
			mapWindow.Close()
		})
		closeBtn.Importance = widget.LowImportance

		// Rebuild control panel with proper close handler
		mapView.buildControlPanel()
	}

	mapWindow.SetContent(mapView)
	mapWindow.Show()
}

// showMapViewDialog opens the map view centered on a specific person.
// This is the standard entry point from menus and context menus.
func showMapViewDialog(w fyne.Window, s *store.Store, personID int64, onNavigate func(int64)) {
	// Get the person
	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Cannot show map view: %w", err), w)
		return
	}

	// Create map window with room for control panel
	mapWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Map View - %s", formatPersonName(*person)))
	mapWindow.Resize(fyne.NewSize(950, 800))  // Taller for control panel
	mapWindow.CenterOnScreen()

	// Create map view widget
	var mapView *MapView
	mapView = NewMapView(s, mapWindow, func(pid int64) {
		onNavigate(pid)
		// Update map view to new person
		if newPerson, err := s.GetPersonByID(pid); err == nil {
			mapView.SetPersonFocus(newPerson)
			mapWindow.SetTitle(fmt.Sprintf("Map View - %s", formatPersonName(*newPerson)))
		}
	})

	// Set initial person BEFORE setting content (so UI builds with correct center)
	mapView.currentPerson = person
	
	// Try to center on person's birth location before building UI
	centered := false
	if person.BirthPlace != "" {
		// Check if already geocoded
		geocode, err := s.GetPlaceGeocode(person.BirthPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			mapView.centerLat = geocode.Latitude
			mapView.centerLon = geocode.Longitude
			mapView.zoomLevel = 10
			centered = true
		} else {
			// Try to geocode it now (this will cache it for future use)
			// Note: This is synchronous and may take 1 second due to rate limiting
			geocode, err := s.GeocodePlace(store.CleanPlaceName(person.BirthPlace))
			if err == nil && geocode != nil {
				mapView.centerLat = geocode.Latitude
				mapView.centerLon = geocode.Longitude
				mapView.zoomLevel = 10
				centered = true
			}
		}
	}
	
	// Try death place if birth didn't work
	if !centered && person.DeathPlace != "" {
		geocode, err := s.GetPlaceGeocode(person.DeathPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			mapView.centerLat = geocode.Latitude
			mapView.centerLon = geocode.Longitude
			mapView.zoomLevel = 10
			centered = true
		} else {
			// Try to geocode it now
			geocode, err := s.GeocodePlace(store.CleanPlaceName(person.DeathPlace))
			if err == nil && geocode != nil {
				mapView.centerLat = geocode.Latitude
				mapView.centerLon = geocode.Longitude
				mapView.zoomLevel = 10
				centered = true
			}
		}
	}
	
	// If still not centered, show a message
	if !centered {
		dialog.ShowInformation("No Location Data", 
			fmt.Sprintf("No geocoded location found for %s.\n\nPlease run Tools → Batch Geocoding first.", 
				formatPersonName(*person)), w)
	}
	
	mapWindow.SetContent(mapView)
	mapWindow.Show()
}

// SetPersonFocus centers the map on a person's locations.
func (mv *MapView) SetPersonFocus(person *store.Person) {
	// Store the current person
	mv.currentPerson = person
	
	// Force reload of markers (clear cache to pick up any changes)
	mv.markers = nil
	
	// Try to center map on person's birth location
	centered := false
	if person.BirthPlace != "" {
		// Try to get geocoded location
		geocode, err := mv.store.GetPlaceGeocode(person.BirthPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			// Center map on birth location
			mv.centerLat = geocode.Latitude
			mv.centerLon = geocode.Longitude
			mv.zoomLevel = 10 // City-level zoom
			centered = true
		}
	}
	
	// If no birth location, try death location
	if !centered && person.DeathPlace != "" {
		geocode, err := mv.store.GetPlaceGeocode(person.DeathPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			mv.centerLat = geocode.Latitude
			mv.centerLon = geocode.Longitude
			mv.zoomLevel = 10
			centered = true
		}
	}
	
	// Update zoom label if it exists
	if mv.zoomLabel != nil {
		mv.zoomLabel.SetText(fmt.Sprintf("Zoom: %d", mv.zoomLevel))
	}
	
	// Update status label if UI is already built
	if mv.statusLabel != nil {
		if person.BirthPlace != "" {
			// Check geocode for debugging
			if geocode, err := mv.store.GetPlaceGeocode(person.BirthPlace); err == nil && geocode != nil {
				mv.statusLabel.SetText(fmt.Sprintf("Viewing: %s (Born: %s at %.4f, %.4f)", 
					formatPersonName(*person), person.BirthPlace, geocode.Latitude, geocode.Longitude))
			} else {
				mv.statusLabel.SetText(fmt.Sprintf("Viewing: %s (Born: %s - not geocoded)", 
					formatPersonName(*person), person.BirthPlace))
			}
		} else {
			mv.statusLabel.SetText(fmt.Sprintf("Viewing: %s", formatPersonName(*person)))
		}
	}
	
	// Refresh map to show new location
	if centered {
		mv.refreshMap()
	}
}

// drawMigrationPaths draws lines connecting life events chronologically for each person.
func (mv *MapView) drawMigrationPaths(markers []*MapMarker, minX, minY int, gridStartX, gridStartY float32, tileSize float32) []fyne.CanvasObject {
	// Group markers by person
	personMarkers := make(map[int64][]*MapMarker)
	for _, m := range markers {
		personMarkers[m.PersonID] = append(personMarkers[m.PersonID], m)
	}
	
	var pathObjects []fyne.CanvasObject
	
	// For each person, draw paths connecting their life events
	for _, markers := range personMarkers {
		if len(markers) < 2 {
			continue // Need at least 2 events to draw a path
		}
		
		// Sort markers chronologically (birth first, then death)
		// Birth markers (type 0) should come before death markers (type 1)
		sortedMarkers := make([]*MapMarker, len(markers))
		copy(sortedMarkers, markers)
		
		// Simple sort: birth -> marriage -> death/current
		birthIdx := -1
		deathIdx := -1
		currentIdx := -1 // For living people's current address
		marriageIndices := []int{}
		
		for i, m := range sortedMarkers {
			switch m.Type {
			case MarkerBirth:
				birthIdx = i
			case MarkerDeath:
				deathIdx = i
			case MarkerMarriage:
				marriageIndices = append(marriageIndices, i)
			case MarkerCurrentAddress:
				currentIdx = i
			}
		}
		
		// Determine end point: death for deceased, current address for living
		endIdx := -1
		if deathIdx >= 0 {
			endIdx = deathIdx
		} else if currentIdx >= 0 {
			endIdx = currentIdx
		}
		
		// Draw path from birth to end point (death or current location)
		if birthIdx >= 0 && endIdx >= 0 {
			birthMarker := sortedMarkers[birthIdx]
			endMarker := sortedMarkers[endIdx]
			
			// Convert lat/lng to screen coordinates
			birthTile := LatLngToTile(birthMarker.Latitude, birthMarker.Longitude, mv.zoomLevel)
			birthOffset := latLngToTilePixelOffset(birthMarker.Latitude, birthMarker.Longitude, mv.zoomLevel)
			birthX := gridStartX + float32((birthTile.X-minX)*int(tileSize)) + birthOffset.X
			birthY := gridStartY + float32((birthTile.Y-minY)*int(tileSize)) + birthOffset.Y
			
			endTile := LatLngToTile(endMarker.Latitude, endMarker.Longitude, mv.zoomLevel)
			endOffset := latLngToTilePixelOffset(endMarker.Latitude, endMarker.Longitude, mv.zoomLevel)
			endX := gridStartX + float32((endTile.X-minX)*int(tileSize)) + endOffset.X
			endY := gridStartY + float32((endTile.Y-minY)*int(tileSize)) + endOffset.Y
			
			// Draw line from birth to end location
			// Use green for living people, blue for deceased
			lineColor := color.RGBA{R: 100, G: 100, B: 255, A: 200} // Blue for deceased
			if endIdx == currentIdx {
				lineColor = color.RGBA{R: 50, G: 200, B: 50, A: 200} // Green for living
			}
			
			line := canvas.NewLine(lineColor)
			line.StrokeWidth = 2
			line.Position1 = fyne.NewPos(birthX, birthY)
			line.Position2 = fyne.NewPos(endX, endY)
			
			pathObjects = append(pathObjects, line)
		}
	}
	
	return pathObjects
}

// mapClickCatcher wraps a canvas object to capture the exact tap position,
// unlike TappableContainer (context_menu.go) whose callback deliberately
// drops the *fyne.PointEvent for its simpler popup-menu use cases.
type mapClickCatcher struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func(pos fyne.Position)
}

func newMapClickCatcher(content fyne.CanvasObject, onTap func(pos fyne.Position)) *mapClickCatcher {
	c := &mapClickCatcher{content: content, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

// CreateRenderer implements fyne.Widget.
func (c *mapClickCatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}

// Tapped implements fyne.Tappable.
func (c *mapClickCatcher) Tapped(e *fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap(e.Position)
	}
}

// screenToLatLng converts a position on the map canvas (in the same
// coordinate space as the most recent render) back to a lat/lng coordinate.
// It inverts the projection used to place tiles and markers
// (LatLngToTile + latLngToTilePixelOffset).
func (mv *MapView) screenToLatLng(pos fyne.Position) (float64, float64) {
	const tileSize = 256.0
	tileX := float64(pos.X-mv.lastGridStartX)/tileSize + float64(mv.lastGridMinX)
	tileY := float64(pos.Y-mv.lastGridStartY)/tileSize + float64(mv.lastGridMinY)
	ll := TileToLatLngF(tileX, tileY, mv.zoomLevel)
	return ll.Lat, ll.Lon
}

// handleMapClick processes a click on the map background for the measure
// tool. It's a no-op when the measure tool isn't active.
func (mv *MapView) handleMapClick(pos fyne.Position) {
	if !mv.measureMode {
		return
	}

	lat, lng := mv.screenToLatLng(pos)
	mv.registerMeasurePoint(lat, lng)
}

// registerMeasurePoint records a measure-tool click at an exact lat/lng.
// Used both for clicks on empty map area (via handleMapClick, which first
// inverts the screen position) and for clicks directly on a marker (which
// already knows its exact coordinates, more precise than hit-testing the
// background under the pin).
func (mv *MapView) registerMeasurePoint(lat, lng float64) {
	point := LatLng{Lat: lat, Lon: lng}

	if mv.measurePointA == nil || mv.measurePointB != nil {
		// Start a fresh measurement.
		mv.measurePointA = &point
		mv.measurePointB = nil
		mv.measureStatusLabel.SetText("Click a second point to measure distance...")
	} else {
		mv.measurePointB = &point
		dist := CalculateDistance(mv.measurePointA.Lat, mv.measurePointA.Lon, point.Lat, point.Lon)
		miles := dist * 0.621371
		mv.measureStatusLabel.SetText(fmt.Sprintf("📏 Distance: %.1f km (%.1f mi)", dist, miles))
	}

	mv.refreshMap()
}

// drawMeasureOverlay draws the measure tool's point markers and connecting
// line, using the same lat/lng-to-pixel projection as markers and paths.
func (mv *MapView) drawMeasureOverlay(minX, minY int, gridStartX, gridStartY float32) []fyne.CanvasObject {
	const tileSize = 256

	toScreen := func(p *LatLng) fyne.Position {
		tileCoord := LatLngToTile(p.Lat, p.Lon, mv.zoomLevel)
		pixelOffset := latLngToTilePixelOffset(p.Lat, p.Lon, mv.zoomLevel)
		x := gridStartX + float32((tileCoord.X-minX)*tileSize) + pixelOffset.X
		y := gridStartY + float32((tileCoord.Y-minY)*tileSize) + pixelOffset.Y
		return fyne.NewPos(x, y)
	}

	newPoint := func(pos fyne.Position) *canvas.Circle {
		const r = 5
		dot := canvas.NewCircle(color.RGBA{R: 255, G: 140, B: 0, A: 255})
		dot.StrokeColor = color.RGBA{R: 0, G: 0, B: 0, A: 255}
		dot.StrokeWidth = 1
		dot.Position1 = fyne.NewPos(pos.X-r, pos.Y-r)
		dot.Position2 = fyne.NewPos(pos.X+r, pos.Y+r)
		return dot
	}

	var objects []fyne.CanvasObject

	if mv.measurePointA != nil {
		posA := toScreen(mv.measurePointA)
		objects = append(objects, newPoint(posA))

		if mv.measurePointB != nil {
			posB := toScreen(mv.measurePointB)
			line := canvas.NewLine(color.RGBA{R: 255, G: 140, B: 0, A: 255})
			line.StrokeWidth = 2
			line.Position1 = posA
			line.Position2 = posB
			objects = append(objects, line, newPoint(posB))
		}
	}

	return objects
}

// calculateAgeAtEvent calculates a person's age at a specific event.
// Returns -1 if dates cannot be parsed.
func calculateAgeAtEvent(birthDate, eventDate string) int {
	birth := parseDateForSort(birthDate)
	event := parseDateForSort(eventDate)
	
	if birth.IsZero() || event.IsZero() {
		return -1
	}
	
	// Calculate age in years
	age := event.Year() - birth.Year()
	
	// Adjust if birthday hasn't occurred yet in event year
	if event.Month() < birth.Month() || (event.Month() == birth.Month() && event.Day() < birth.Day()) {
		age--
	}
	
	// Return -1 if age would be negative (event before birth)
	if age < 0 {
		return -1
	}
	
	return age
}

// showExportDialog displays the map export options dialog.
func (mv *MapView) showExportDialog() {
	// Create export options
	pngBtn := widget.NewButton("Export as PNG Image", func() {
		mv.exportMapAsPNG()
	})
	pngBtn.Importance = widget.HighImportance
	
	pdfBtn := widget.NewButton("Export as PDF Document", func() {
		mv.exportMapAsPDF()
	})
	pdfBtn.Importance = widget.HighImportance
	
	// Info label
	infoLabel := widget.NewLabel("Export the current map view with all visible markers and paths.")
	infoLabel.Wrapping = fyne.TextWrapWord
	
	// Dialog content
	content := container.NewVBox(
		widget.NewLabelWithStyle("Export Map", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		infoLabel,
		widget.NewLabel(""),
		pngBtn,
		pdfBtn,
	)
	
	// Show dialog
	exportDialog := dialog.NewCustom("Export Map", "Cancel", content, mv.window)
	exportDialog.Resize(fyne.NewSize(400, 250))
	exportDialog.Show()
}

// exportMapAsPNG exports the current map view as a PNG image.
func (mv *MapView) exportMapAsPNG() {
	// Create file save dialog
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}

		runWithProgress(mv.window, "Exporting Map", "Downloading map tiles and rendering...\nThis may take 10-30 seconds.",
			func() error {
				// writer must stay open for the whole capture, then close -
				// closing it via an outer defer would race with (and likely
				// precede) this background write.
				defer writer.Close()
				return mv.captureMapToFile(writer)
			},
			func(err error) {
				if err != nil {
					dialog.ShowError(fmt.Errorf("Failed to export map: %v", err), mv.window)
					return
				}
				dialog.ShowInformation("Export Complete", "Map exported successfully as PNG!", mv.window)
			})
	}, mv.window)
	
	saveDialog.SetFileName("map_export.png")
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
	saveDialog.Show()
}

// exportMapAsPDF exports the current map view as a PDF document.
func (mv *MapView) exportMapAsPDF() {
	// Create file save dialog
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		
		// First, create a temporary PNG
		tmpFile, err := os.CreateTemp("", "map_export_*.png")
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create temporary file: %v", err), mv.window)
			return
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()
		
		// Capture map to temp PNG
		err = mv.captureMapToFile(tmpFile)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to capture map: %v", err), mv.window)
			return
		}
		tmpFile.Close()
		
		// Convert PNG to PDF
		err = mv.convertPNGtoPDF(tmpFile.Name(), writer)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create PDF: %v", err), mv.window)
			return
		}
		
		dialog.ShowInformation("Export Complete", "Map exported successfully as PDF!", mv.window)
	}, mv.window)
	
	saveDialog.SetFileName("map_export.pdf")
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".pdf"}))
	saveDialog.Show()
}

// captureMapToFile captures the current map view and writes it as PNG to the provided writer.
func (mv *MapView) captureMapToFile(writer interface{ Write([]byte) (int, error) }) error {
	// Use a reasonable fixed size for export (Full HD)
	width := 1920
	height := 1080
	
	// Create a new image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Fill with light background color
	bgColor := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
	
	// Calculate tile grid for current view
	tileSize := 256
	centerTile := LatLngToTile(mv.centerLat, mv.centerLon, mv.zoomLevel)
	
	// Calculate how many tiles we need to cover the canvas
	numTilesX := (width / tileSize) + 2
	numTilesY := (height / tileSize) + 2
	
	// Calculate tile range
	minX := centerTile.X - numTilesX/2
	maxX := centerTile.X + numTilesX/2
	minY := centerTile.Y - numTilesY/2
	maxY := centerTile.Y + numTilesY/2
	
	// Download tiles in parallel for better performance
	type tileResult struct {
		x, y int
		img  image.Image
	}
	
	tileChan := make(chan tileResult, (maxX-minX+1)*(maxY-minY+1))
	var wg sync.WaitGroup
	
	// Limit concurrent downloads to avoid overwhelming the server
	semaphore := make(chan struct{}, 6)
	
	for tileX := minX; tileX <= maxX; tileX++ {
		for tileY := minY; tileY <= maxY; tileY++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()
				
				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				
				// Download tile
				tileImg, err := mv.downloadRawTile(x, y, mv.zoomLevel)
				if err == nil && tileImg != nil {
					tileChan <- tileResult{x, y, tileImg}
				}
			}(tileX, tileY)
		}
	}
	
	// Close channel when all downloads complete
	go func() {
		wg.Wait()
		close(tileChan)
	}()
	
	// Composite downloaded tiles into main image
	for result := range tileChan {
		posX := (result.x - minX) * tileSize
		posY := (result.y - minY) * tileSize
		destRect := image.Rect(posX, posY, posX+tileSize, posY+tileSize)
		draw.Draw(img, destRect, result.img, image.Point{}, draw.Over)
	}
	
	// Draw migration paths (underneath markers)
	if mv.showPaths && len(mv.markers) > 0 {
		mv.drawExportPaths(img, minX, minY, tileSize)
	}
	
	// Draw markers on top
	mv.drawExportMarkers(img, minX, minY, tileSize)
	
	// Encode as PNG
	return png.Encode(writer, img)
}

// downloadRawTile downloads a tile and returns it as image.Image.
func (mv *MapView) downloadRawTile(x, y, zoom int) (image.Image, error) {
	tileURL := GetTileURL(x, y, zoom)
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "KrankyBearGenealogy/1.8.0 (Genealogy Research Tool)")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tile server returned status %d", resp.StatusCode)
	}
	
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// drawExportPaths draws migration paths on the export image.
func (mv *MapView) drawExportPaths(img *image.RGBA, minX, minY, tileSize int) {
	// Group markers by person
	pathsByPerson := make(map[int64][]*MapMarker)
	for _, m := range mv.markers {
		pathsByPerson[m.PersonID] = append(pathsByPerson[m.PersonID], m)
	}
	
	for _, personMarkers := range pathsByPerson {
		if len(personMarkers) < 2 {
			continue
		}
		
		// Draw lines between chronological events
		for i := 0; i < len(personMarkers)-1; i++ {
			startMarker := personMarkers[i]
			endMarker := personMarkers[i+1]
			
			// Calculate pixel positions
			startTile := LatLngToTile(startMarker.Latitude, startMarker.Longitude, mv.zoomLevel)
			startOffset := latLngToTilePixelOffset(startMarker.Latitude, startMarker.Longitude, mv.zoomLevel)
			startX := (startTile.X-minX)*tileSize + int(startOffset.X)
			startY := (startTile.Y-minY)*tileSize + int(startOffset.Y)
			
			endTile := LatLngToTile(endMarker.Latitude, endMarker.Longitude, mv.zoomLevel)
			endOffset := latLngToTilePixelOffset(endMarker.Latitude, endMarker.Longitude, mv.zoomLevel)
			endX := (endTile.X-minX)*tileSize + int(endOffset.X)
			endY := (endTile.Y-minY)*tileSize + int(endOffset.Y)
			
			// Determine line color
			lineColor := color.RGBA{R: 100, G: 100, B: 255, A: 200} // Blue for deceased
			if i == len(personMarkers)-2 && endMarker.Type == MarkerCurrentAddress {
				lineColor = color.RGBA{R: 50, G: 200, B: 50, A: 200} // Green for living
			}
			
			// Draw line
			drawMapLine(img, startX, startY, endX, endY, lineColor)
		}
	}
}

// drawExportMarkers draws markers on the export image.
func (mv *MapView) drawExportMarkers(img *image.RGBA, minX, minY, tileSize int) {
	for _, marker := range mv.markers {
		// Filter markers based on visibility settings
		if (marker.Type == MarkerBirth && !mv.showBirths) ||
			(marker.Type == MarkerDeath && !mv.showDeaths) ||
			(marker.Type == MarkerMarriage && !mv.showMarriages) ||
			(marker.Type == MarkerCurrentAddress && !mv.showBirths) {
			continue
		}
		
		// Calculate marker position
		markerTile := LatLngToTile(marker.Latitude, marker.Longitude, mv.zoomLevel)
		markerOffset := latLngToTilePixelOffset(marker.Latitude, marker.Longitude, mv.zoomLevel)
		markerX := (markerTile.X-minX)*tileSize + int(markerOffset.X)
		markerY := (markerTile.Y-minY)*tileSize + int(markerOffset.Y)
		
		// Draw marker
		markerRadius := 8
		markerColor := getMapMarkerColor(marker.Type)
		drawMapFilledCircle(img, markerX, markerY, markerRadius, markerColor)
		
		// Draw border
		borderColor := color.RGBA{R: 0, G: 0, B: 0, A: 255}
		drawMapCircle(img, markerX, markerY, markerRadius, borderColor)
	}
}

// drawMapLine draws a line between two points using Bresenham's algorithm.
func drawMapLine(img *image.RGBA, x0, y0, x1, y1 int, col color.Color) {
	dx := absInt(x1 - x0)
	dy := absInt(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	
	for {
		if x0 >= 0 && x0 < img.Bounds().Dx() && y0 >= 0 && y0 < img.Bounds().Dy() {
			img.Set(x0, y0, col)
			// Draw thicker line (2px wide)
			if x0+1 < img.Bounds().Dx() {
				img.Set(x0+1, y0, col)
			}
			if y0+1 < img.Bounds().Dy() {
				img.Set(x0, y0+1, col)
			}
		}
		
		if x0 == x1 && y0 == y1 {
			break
		}
		
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// drawMapFilledCircle draws a filled circle at the given position.
func drawMapFilledCircle(img *image.RGBA, cx, cy, radius int, col color.Color) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				px, py := cx+x, cy+y
				if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
					img.Set(px, py, col)
				}
			}
		}
	}
}

// drawMapCircle draws a circle outline at the given position.
func drawMapCircle(img *image.RGBA, cx, cy, radius int, col color.Color) {
	x, y, d := radius, 0, 1-radius
	for x >= y {
		points := []struct{ x, y int }{
			{cx + x, cy + y}, {cx + y, cy + x},
			{cx - x, cy + y}, {cx - y, cy + x},
			{cx + x, cy - y}, {cx + y, cy - x},
			{cx - x, cy - y}, {cx - y, cy - x},
		}
		for _, p := range points {
			if p.x >= 0 && p.x < img.Bounds().Dx() && p.y >= 0 && p.y < img.Bounds().Dy() {
				img.Set(p.x, p.y, col)
			}
		}
		y++
		if d <= 0 {
			d += 2*y + 1
		} else {
			x--
			d += 2*(y-x) + 1
		}
	}
}

// absInt returns the absolute value of an integer.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// getMapMarkerColor returns the color for a given marker type.
func getMapMarkerColor(markerType MarkerType) color.Color {
	switch markerType {
	case MarkerBirth:
		return color.RGBA{R: 50, G: 200, B: 50, A: 255} // Green
	case MarkerDeath:
		return color.RGBA{R: 50, G: 50, B: 50, A: 255} // Dark gray
	case MarkerMarriage:
		return color.RGBA{R: 255, G: 150, B: 200, A: 255} // Pink
	case MarkerCurrentAddress:
		return color.RGBA{R: 100, G: 255, B: 100, A: 255} // Light green
	case MarkerEvent:
		return color.RGBA{R: 150, G: 80, B: 220, A: 255} // Purple
	default:
		return color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray
	}
}


// convertPNGtoPDF converts a PNG file to PDF.
func (mv *MapView) convertPNGtoPDF(pngPath string, writer interface{ Write([]byte) (int, error) }) error {
	// Create PDF
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	
	// Get page dimensions
	pageWidth, pageHeight := pdf.GetPageSize()
	marginX, marginY := 10.0, 10.0
	maxWidth := pageWidth - 2*marginX
	maxHeight := pageHeight - 2*marginY
	
	// Add title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(pageWidth-2*marginX, 10, "Geographic Map Export", "", 1, "C", false, 0, "")
	pdf.Ln(5)
	
	// Add map image
	imageOptions := gofpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   true,
	}
	
	// Calculate scaled dimensions to fit page
	pdf.ImageOptions(pngPath, marginX, marginY+15, maxWidth, maxHeight-15, false, imageOptions, 0, "")
	
	// Add footer with map details
	pdf.SetY(pageHeight - 15)
	pdf.SetFont("Arial", "I", 10)
	viewModeText := fmt.Sprintf("View: %s", mv.viewMode)
	if mv.currentPerson != nil && mv.viewMode == "person" {
		viewModeText = fmt.Sprintf("Person: %s %s", mv.currentPerson.GivenName, mv.currentPerson.Surname)
	}
	pdf.CellFormat(pageWidth-2*marginX, 5, viewModeText, "", 1, "L", false, 0, "")
	
	markerInfo := fmt.Sprintf("Markers: %d total", len(mv.markers))
	if mv.markerStatsLabel != nil && mv.markerStatsLabel.Text != "" {
		markerInfo = mv.markerStatsLabel.Text
	}
	pdf.CellFormat(pageWidth-2*marginX, 5, markerInfo, "", 1, "L", false, 0, "")
	
	// Write PDF to output
	return pdf.Output(writer)
}
