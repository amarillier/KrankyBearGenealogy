package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// showGeocodingToolDialog shows the batch geocoding tool.
func showGeocodingToolDialog(w fyne.Window, s *store.Store) {
	// Get all unique places
	allPlaces, err := s.GetAllUniquePlaces()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get places: %w", err), w)
		return
	}

	// Get geocoded places
	geocoded, err := s.GetGeocodedPlaces()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get geocoded places: %w", err), w)
		return
	}

	geocodedCount := len(geocoded)
	totalCount := len(allPlaces)
	unmappedCount := totalCount - geocodedCount

	// Create dialog content
	infoLabel := widget.NewLabel(fmt.Sprintf(
		"Total places: %d\nGeocoded: %d\nNot geocoded: %d",
		totalCount, geocodedCount, unmappedCount))

	statusLabel := widget.NewLabel("Ready to geocode")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	var startBtn *widget.Button
	startBtn = widget.NewButton("Start Batch Geocoding", func() {
		// Disable button during processing
		startBtn.Disable()
		progressBar.Show()
		statusLabel.SetText("Geocoding in progress...")

		go func() {
			unmapped, _ := s.GetUnmappedPlaces()
			total := len(unmapped)

			for i, place := range unmapped {
				// Update UI on main thread
				currentI := i
				currentPlace := place
				fyne.Do(func() {
					progressBar.SetValue(float64(currentI) / float64(total))
					statusLabel.SetText(fmt.Sprintf("Geocoding %d/%d: %s", currentI+1, total, currentPlace))
				})

				// Geocode the place
				cleanPlace := store.CleanPlaceName(place)
				_, err := s.GeocodePlace(cleanPlace)
				if err != nil {
					// Continue on error, will be marked as failed in cache
					continue
				}

				// Respect rate limit (1 request per second for Nominatim)
				time.Sleep(1 * time.Second)
			}

			// Update UI on main thread
			fyne.Do(func() {
				progressBar.SetValue(1.0)
				statusLabel.SetText(fmt.Sprintf("Completed! Geocoded %d places", total))
				startBtn.Enable()

				// Refresh counts
				newGeocoded, _ := s.GetGeocodedPlaces()
				infoLabel.SetText(fmt.Sprintf(
					"Total places: %d\nGeocoded: %d\nNot geocoded: %d",
					totalCount, len(newGeocoded), totalCount-len(newGeocoded)))
			})
		}()
	})

	if unmappedCount == 0 {
		startBtn.Disable()
		statusLabel.SetText("All places are already geocoded!")
	}

	viewResultsBtn := widget.NewButton("View Geocoded Places", func() {
		showGeocodedPlacesReport(w, s)
	})

	viewUnmappedBtn := widget.NewButton("View Unmapped Places", func() {
		showUnmappedPlacesReport(w, s)
	})

	closeBtn := widget.NewButton("Close", func() {})

	content := container.NewVBox(
		widget.NewLabel("Batch Geocoding Tool"),
		widget.NewSeparator(),
		infoLabel,
		widget.NewLabel("\nThis tool will geocode all places in your database using OpenStreetMap's Nominatim service."),
		widget.NewLabel("Rate limit: 1 request per second (this may take a while for large databases)."),
		widget.NewSeparator(),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		container.NewHBox(startBtn, viewResultsBtn, viewUnmappedBtn),
	)

	d := dialog.NewCustom("Batch Geocoding", "Close", content, w)
	closeBtn.OnTapped = func() {
		d.Hide()
	}
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// showGeocodedPlacesReport shows all successfully geocoded places.
func showGeocodedPlacesReport(w fyne.Window, s *store.Store) {
	places, err := s.GetGeocodedPlaces()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get geocoded places: %w", err), w)
		return
	}

	if len(places) == 0 {
		dialog.ShowInformation("Geocoded Places", "No places have been geocoded yet.", w)
		return
	}

	// Create report content
	content := fmt.Sprintf("Geocoded Places Report\n\n")
	content += fmt.Sprintf("Total geocoded: %d places\n\n", len(places))

	for _, pg := range places {
		content += fmt.Sprintf("📍 %s\n", pg.PlaceName)
		content += fmt.Sprintf("   Coordinates: %.6f, %.6f\n", pg.Latitude, pg.Longitude)
		if pg.City != "" {
			content += fmt.Sprintf("   City: %s\n", pg.City)
		}
		if pg.StateProvince != "" {
			content += fmt.Sprintf("   State: %s\n", pg.StateProvince)
		}
		if pg.Country != "" {
			content += fmt.Sprintf("   Country: %s\n", pg.Country)
		}
		content += "\n"
	}

	// Create scrollable report
	reportText := widget.NewLabel(content)
	reportText.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(reportText)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	dialog.ShowCustom("Geocoded Places", "Close", scroll, w)
}

// showUnmappedPlacesReport shows places that haven't been geocoded.
func showUnmappedPlacesReport(w fyne.Window, s *store.Store) {
	unmapped, err := s.GetUnmappedPlaces()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get unmapped places: %w", err), w)
		return
	}

	if len(unmapped) == 0 {
		dialog.ShowInformation("Unmapped Places", "All places have been geocoded!", w)
		return
	}

	// Create report content
	content := fmt.Sprintf("Unmapped Places Report\n\n")
	content += fmt.Sprintf("Total unmapped: %d places\n\n", len(unmapped))

	for i, place := range unmapped {
		content += fmt.Sprintf("%d. %s\n", i+1, place)
	}

	// Create scrollable report
	reportText := widget.NewLabel(content)
	reportText.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(reportText)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	dialog.ShowCustom("Unmapped Places", "Close", scroll, w)
}
