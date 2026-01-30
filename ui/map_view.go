package ui

import (
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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
	markers       []*MapMarker

	// UI components
	content         *fyne.Container
	controlPanel    *fyne.Container
	zoomInBtn       *widget.Button
	zoomOutBtn      *widget.Button
	zoomLabel       *widget.Label
	statusLabel     *widget.Label
	loadingLabel    *widget.Label
	viewModeSelect  *widget.Select
	surnameEntry    *widget.Entry
	startYearEntry  *widget.Entry
	endYearEntry    *widget.Entry
	livingSelect    *widget.Select
	generationCheck *widget.Check
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
	bg.Resize(fyne.NewSize(800, 600))
	mv.mapCanvas = container.NewWithoutLayout(bg, container.NewCenter(mv.loadingLabel))

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

	// Main layout - map canvas directly in center (fixed size, no scrolling for now)
	mv.content = container.NewBorder(
		mv.controlPanel,  // Top
		statusBar,        // Bottom
		nil,              // Left
		nil,              // Right
		mv.mapCanvas,     // Center (fixed 800x600)
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

	zoomControls := container.NewHBox(
		widget.NewLabel("🔍"),
		mv.zoomInBtn,
		mv.zoomOutBtn,
		mv.zoomLabel,
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

	filterControls := container.NewHBox(
		widget.NewLabel("Show:"),
		showBirthsCheck,
		centerBirthBtn,
		showDeathsCheck,
		centerDeathBtn,
		showMarriagesCheck,
		centerMarriageBtn,
	)

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

	// Assemble control panel
	mv.controlPanel = container.NewBorder(
		nil,
		nil,
		container.NewHBox(zoomControls, layout.NewSpacer(), filterControls),
		container.NewHBox(helpBtn, closeBtn),
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
	viewportWidth := 800
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
			dialog.ShowInformation("Map Unavailable",
				"Map tiles could not be loaded.\n\n"+
				"This feature requires an active internet connection to download map data from OpenStreetMap.\n\n"+
				"Please check your network connection and try again.",
				mv.window)
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
	
	// We want the marker centered in 800x600 viewport at (400, 300)
	// Position the tile grid so marker ends up there
	gridStartX := 400 - markerRelX
	gridStartY := 300 - markerRelY
	
	// Canvas is fixed at 800x600
	canvasWidth := float32(800)
	canvasHeight := float32(600)
	
	// Create grid container
	gridObjects := make([]fyne.CanvasObject, 0)
	
	// Add sized background
	bg := canvas.NewRectangle(color.RGBA{R: 200, G: 220, B: 240, A: 255})
	bg.Resize(fyne.NewSize(canvasWidth, canvasHeight))
	gridObjects = append(gridObjects, bg)
	
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
		
		// Create marker widget
		markerWidget := CreateMarkerWidget(m, func() {
			mv.showMarkerPopup(m)
		})
		
		// Position marker
		markerWidget.Resize(fyne.NewSize(20, 20))
		markerWidget.Move(fyne.NewPos(markerX, markerY))
		
		gridObjects = append(gridObjects, markerWidget)
	}
	
	// Set canvas size to fit all tiles
	mv.mapCanvas.Resize(fyne.NewSize(canvasWidth, canvasHeight))
	
	// Update canvas objects
	mv.mapCanvas.Objects = gridObjects
	mv.mapCanvas.Refresh()
}

// loadMarkers loads all markers from the database.
func (mv *MapView) loadMarkers() {
	// Check if we have a current person (show only their markers)
	if mv.currentPerson != nil {
		markers, err := GetMarkersForPerson(mv.store, mv.currentPerson.ID)
		if err == nil {
			mv.markers = markers
		}
	} else {
		// Show all markers
		markers, err := GetAllMarkers(mv.store)
		if err == nil {
			mv.markers = markers
		}
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
		}
		
		if include {
			filtered = append(filtered, marker)
		}
	}
	
	return filtered
}

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
	
	switch marker.Type {
	case MarkerBirth:
		message = fmt.Sprintf("Birth of %s\n\nDate: %s\nPlace: %s",
			marker.PersonName, marker.EventDate, marker.Place)
	case MarkerDeath:
		message = fmt.Sprintf("Death of %s\n\nDate: %s\nPlace: %s",
			marker.PersonName, marker.EventDate, marker.Place)
	case MarkerMarriage:
		message = fmt.Sprintf("Marriage\n\n%s\n& %s\n\nDate: %s\nPlace: %s",
			marker.PersonName, marker.SpouseName, marker.EventDate, marker.Place)
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
  • Pan: Click and drag (coming soon)
  • Markers: Click to view person details

📍 Marker Types:
  • 🟢 Green: Birth locations
  • ⚫ Gray: Death locations
  • 💒 Pink: Marriage locations

🔍 Filters:
  • Toggle checkboxes to show/hide event types
  • More filters coming soon

💡 Tips:
  • Right-click any person → "View on Map"
  • Map uses OpenStreetMap tiles (free, no API key)`

	dialog.ShowInformation("Map View Help", helpText, mv.window)
}

// ShowMapWindow opens the map view in a new window.
func ShowMapWindow(s *store.Store, parentWindow fyne.Window, onNavigate func(personID int64)) {
	mapView := NewMapView(s, parentWindow, onNavigate)

	// Create new window
	mapWindow := fyne.CurrentApp().NewWindow("Map View - KrankyBear Genealogy")
	mapWindow.Resize(fyne.NewSize(900, 700))
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

	// Create map window
	mapWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Map View - %s", formatPersonName(*person)))
	mapWindow.Resize(fyne.NewSize(900, 700))
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
