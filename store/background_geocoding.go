package store

import (
	"log"
	"sync"
	"time"
)

// BackgroundGeocoder handles automatic background geocoding of places
type BackgroundGeocoder struct {
	store    *Store
	running  bool
	stopChan chan bool
	mu       sync.Mutex
}

// NewBackgroundGeocoder creates a new background geocoder
func NewBackgroundGeocoder(s *Store) *BackgroundGeocoder {
	return &BackgroundGeocoder{
		store:    s,
		stopChan: make(chan bool),
	}
}

// Start begins background geocoding of un-geocoded places
func (bg *BackgroundGeocoder) Start() {
	bg.mu.Lock()
	if bg.running {
		bg.mu.Unlock()
		return // Already running
	}
	bg.running = true
	bg.mu.Unlock()

	go bg.run()
}

// Stop stops the background geocoding process
func (bg *BackgroundGeocoder) Stop() {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	
	if !bg.running {
		return
	}
	
	bg.running = false
	bg.stopChan <- true
}

// run is the main background geocoding loop
func (bg *BackgroundGeocoder) run() {
	log.Println("Background geocoding service started")
	
	// Wait 10 seconds after app start before beginning
	select {
	case <-time.After(10 * time.Second):
		// Continue
	case <-bg.stopChan:
		log.Println("Background geocoding service stopped")
		return
	}
	
	// Geocode un-geocoded places
	bg.geocodeUngecodedPlaces()
	
	log.Println("Background geocoding service completed initial pass")
	
	// Check again every 30 minutes for new places
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			log.Println("Background geocoding: checking for new places")
			bg.geocodeUngecodedPlaces()
		case <-bg.stopChan:
			log.Println("Background geocoding service stopped")
			return
		}
	}
}

// geocodeUngecodedPlaces finds and geocodes places that haven't been geocoded yet
func (bg *BackgroundGeocoder) geocodeUngecodedPlaces() {
	// Get all unique places from the database
	places := bg.getUniqueUngecodedPlaces()
	
	if len(places) == 0 {
		log.Println("Background geocoding: no new places to geocode")
		return
	}
	
	log.Printf("Background geocoding: found %d places to geocode\n", len(places))
	
	// Geocode each place (with rate limiting)
	successCount := 0
	failCount := 0
	
	for i, place := range places {
		// Check if we should stop
		select {
		case <-bg.stopChan:
			log.Printf("Background geocoding: stopped after %d/%d places", i, len(places))
			return
		default:
			// Continue
		}
		
		// Try to geocode the place
		_, err := bg.store.GeocodePlace(place)
		if err != nil {
			failCount++
			log.Printf("Background geocoding: failed to geocode '%s': %v", place, err)
		} else {
			successCount++
			log.Printf("Background geocoding: successfully geocoded '%s' (%d/%d)", place, i+1, len(places))
		}
		
		// Rate limiting is already built into GeocodePlace (1 second)
		// No need to add extra delays here
	}
	
	log.Printf("Background geocoding: completed - %d successful, %d failed", successCount, failCount)
}

// getUniqueUngecodedPlaces returns a list of unique place names that haven't been geocoded
func (bg *BackgroundGeocoder) getUniqueUngecodedPlaces() []string {
	placeMap := make(map[string]bool)
	
	// Query all unique places from persons table
	rows, err := bg.store.DB.Query(`
		SELECT DISTINCT place FROM (
			SELECT birth_place AS place FROM persons WHERE birth_place != '' AND birth_place IS NOT NULL
			UNION
			SELECT death_place AS place FROM persons WHERE death_place != '' AND death_place IS NOT NULL
			UNION
			SELECT CASE 
				WHEN address != '' AND city != '' AND state != '' AND country != '' 
					THEN address || ', ' || city || ', ' || state || ', ' || country
				WHEN city != '' AND state != '' AND country != '' 
					THEN city || ', ' || state || ', ' || country
				WHEN city != '' AND country != '' 
					THEN city || ', ' || country
				WHEN city != '' 
					THEN city
				ELSE NULL
			END AS place
			FROM persons 
			WHERE is_living = 1 AND city != '' AND city IS NOT NULL
		)
		WHERE place NOT IN (
			SELECT place_name FROM place_geocodes WHERE geocode_status = 'success'
		)
	`)
	
	if err != nil {
		log.Printf("Background geocoding: error querying places: %v", err)
		return nil
	}
	defer rows.Close()
	
	var places []string
	for rows.Next() {
		var place string
		if err := rows.Scan(&place); err == nil && place != "" {
			// Skip unknown/placeholder places
			if isUnknownPlaceName(place) {
				continue
			}
			
			if !placeMap[place] {
				placeMap[place] = true
				places = append(places, place)
			}
		}
	}
	
	// Also get marriage places from relationships
	rows2, err := bg.store.DB.Query(`
		SELECT DISTINCT marriage_place 
		FROM relationships 
		WHERE marriage_place != '' AND marriage_place IS NOT NULL
		AND marriage_place NOT IN (
			SELECT place_name FROM place_geocodes WHERE geocode_status = 'success'
		)
	`)
	
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var place string
			if err := rows2.Scan(&place); err == nil && place != "" {
				if !isUnknownPlaceName(place) && !placeMap[place] {
					placeMap[place] = true
					places = append(places, place)
				}
			}
		}
	}
	
	return places
}

// isUnknownPlaceName checks if a place name is a placeholder/unknown
func isUnknownPlaceName(place string) bool {
	lower := ""
	for _, r := range place {
		if r >= 'A' && r <= 'Z' {
			lower += string(r + 32)
		} else {
			lower += string(r)
		}
	}
	
	// Check for common placeholder text
	unknownPatterns := []string{"unknown", "?", "n/a", "none", "not known", "tbd"}
	for _, pattern := range unknownPatterns {
		if lower == pattern {
			return true
		}
	}
	
	return false
}
