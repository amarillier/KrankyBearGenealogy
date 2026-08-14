package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// MarkerType represents the type of life event marker.
type MarkerType int

const (
	MarkerBirth MarkerType = iota
	MarkerDeath
	MarkerMarriage
	MarkerCurrentAddress // For living people's current location
	MarkerEvent          // Generic life event (immigration, occupation, etc.)
)

// MapMarker represents a marker on the map for a life event.
type MapMarker struct {
	Type       MarkerType
	Latitude   float64
	Longitude  float64
	PersonID   int64
	PersonName string
	EventDate  string
	Place      string
	Generation int // Generation relative to reference person (0 = same, negative = ancestors, positive = descendants)

	// Additional info for marriages
	SpouseID   int64
	SpouseName string

	// EventLabel holds the life event's type (e.g. "Immigration") for
	// MarkerEvent markers, since none of the other fields fit.
	EventLabel string

	// EventDateEnd is optionally set for MarkerEvent markers whose event has
	// a duration (e.g. military service), in addition to EventDate as the
	// start.
	EventDateEnd string
}

// MarkerStyle defines the visual appearance of a marker.
type MarkerStyle struct {
	Color  color.Color
	Symbol string
	Size   float32
}

// GetMarkerStyle returns the style for a marker type.
func GetMarkerStyle(markerType MarkerType) MarkerStyle {
	switch markerType {
	case MarkerBirth:
		return MarkerStyle{
			Color:  color.RGBA{R: 50, G: 200, B: 50, A: 255}, // Green
			Symbol: "🟢",
			Size:   20,
		}
	case MarkerDeath:
		return MarkerStyle{
			Color:  color.RGBA{R: 80, G: 80, B: 80, A: 255}, // Gray
			Symbol: "⚫",
			Size:   20,
		}
	case MarkerMarriage:
		return MarkerStyle{
			Color:  color.RGBA{R: 255, G: 100, B: 150, A: 255}, // Pink
			Symbol: "💒",
			Size:   20,
		}
	case MarkerCurrentAddress:
		return MarkerStyle{
			Color:  color.RGBA{R: 50, G: 200, B: 50, A: 255}, // Green (same as birth)
			Symbol: "🏠",
			Size:   20,
		}
	case MarkerEvent:
		return MarkerStyle{
			Color:  color.RGBA{R: 150, G: 80, B: 220, A: 255}, // Purple
			Symbol: "📅",
			Size:   20,
		}
	default:
		return MarkerStyle{
			Color:  color.RGBA{R: 100, G: 100, B: 100, A: 255},
			Symbol: "📍",
			Size:   20,
		}
	}
}

// CreateMarkerWidget creates a clickable marker widget.
func CreateMarkerWidget(marker *MapMarker, onClick func(), useGenerationColor bool) fyne.CanvasObject {
	if useGenerationColor {
		// Use generation-based color (Phase 5)
		genColor := GetGenerationColor(marker.Generation)
		circle := canvas.NewCircle(genColor)
		circle.Resize(fyne.NewSize(20, 20))
		
		// Wrap in a tappable container
		tappable := NewTappableContainer(circle, onClick, nil)
		return tappable
	}
	
	// Use default marker style (emoji)
	style := GetMarkerStyle(marker.Type)
	
	// Create marker icon
	markerLabel := widget.NewLabel(style.Symbol)
	markerLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	// Wrap in a tappable container that doesn't block mouse events like buttons do
	tappable := NewTappableContainer(markerLabel, onClick, nil)
	
	return tappable
}

// GetMarkersForPerson fetches all life event markers for a person.
func GetMarkersForPerson(s *store.Store, personID int64) ([]*MapMarker, error) {
	person, err := s.GetPersonByID(personID)
	if err != nil {
		return nil, err
	}
	
	markers := []*MapMarker{}
	
	// Birth marker - skip if place is empty or placeholder text
	if person.BirthPlace != "" && !isUnknownPlace(person.BirthPlace) {
		geocode, err := s.GetPlaceGeocode(person.BirthPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			markers = append(markers, &MapMarker{
				Type:       MarkerBirth,
				Latitude:   geocode.Latitude,
				Longitude:  geocode.Longitude,
				PersonID:   person.ID,
				PersonName: formatPersonName(*person),
				EventDate:  person.BirthDate,
				Place:      person.BirthPlace,
			})
		}
	}
	
	// Death marker - skip if place is empty or placeholder text
	if person.DeathPlace != "" && !isUnknownPlace(person.DeathPlace) {
		geocode, err := s.GetPlaceGeocode(person.DeathPlace)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			markers = append(markers, &MapMarker{
				Type:       MarkerDeath,
				Latitude:   geocode.Latitude,
				Longitude:  geocode.Longitude,
				PersonID:   person.ID,
				PersonName: formatPersonName(*person),
				EventDate:  person.DeathDate,
				Place:      person.DeathPlace,
			})
		}
	}
	
	// Marriage markers - get all spouses/marriages for this person
	spouses, err := s.GetSpouses(person.ID)
	if err == nil {
		for _, spouse := range spouses {
			if spouse.MarriagePlace != "" && !isUnknownPlace(spouse.MarriagePlace) {
				geocode, err := s.GetPlaceGeocode(spouse.MarriagePlace)
				if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
					markers = append(markers, &MapMarker{
						Type:       MarkerMarriage,
						Latitude:   geocode.Latitude,
						Longitude:  geocode.Longitude,
						PersonID:   person.ID,
						PersonName: formatPersonName(*person),
						SpouseID:   spouse.Person.ID,
						SpouseName: formatPersonName(spouse.Person),
						EventDate:  spouse.MarriageDate,
						Place:      spouse.MarriagePlace,
					})
				}
			}
		}
	}
	
	// Current address for living people - use person's City field
	if person.IsLiving && person.City != "" && !isUnknownPlace(person.City) {
		// Combine address fields for better geocoding
		parts := []string{}
		if person.Address != "" {
			parts = append(parts, person.Address)
		}
		if person.City != "" {
			parts = append(parts, person.City)
		}
		if person.State != "" {
			parts = append(parts, person.State)
		}
		if person.Country != "" {
			parts = append(parts, person.Country)
		}
		
		addressToGeocode := strings.Join(parts, ", ")
		
		geocode, err := s.GetPlaceGeocode(addressToGeocode)
		if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
			markers = append(markers, &MapMarker{
				Type:       MarkerCurrentAddress,
				Latitude:   geocode.Latitude,
				Longitude:  geocode.Longitude,
				PersonID:   person.ID,
				PersonName: formatPersonName(*person),
				EventDate:  "Present",
				Place:      addressToGeocode,
			})
		}
	}

	// Life Events - generic one-off events (immigration, occupation, etc.)
	if events, err := s.GetEvents(person.ID); err == nil {
		for _, event := range events {
			if event.Place == "" || isUnknownPlace(event.Place) {
				continue
			}
			geocode, err := s.GetPlaceGeocode(event.Place)
			if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
				markers = append(markers, &MapMarker{
					Type:         MarkerEvent,
					Latitude:     geocode.Latitude,
					Longitude:    geocode.Longitude,
					PersonID:     person.ID,
					PersonName:   formatPersonName(*person),
					EventDate:    event.Date,
					EventDateEnd: event.DateEnd,
					Place:        event.Place,
					EventLabel:   event.Type,
				})
			}
		}
	}

	return markers, nil
}

// GetAllMarkers fetches all life event markers from the database.
func GetAllMarkers(s *store.Store) ([]*MapMarker, error) {
	markers := []*MapMarker{}
	
	// Get all people
	people, err := s.GetAllPeople()
	if err != nil {
		return nil, err
	}
	
	// Collect birth markers - skip unknown/placeholder places
	for _, person := range people {
		if person.BirthPlace != "" && !isUnknownPlace(person.BirthPlace) {
			geocode, err := s.GetPlaceGeocode(person.BirthPlace)
			if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
				markers = append(markers, &MapMarker{
					Type:       MarkerBirth,
					Latitude:   geocode.Latitude,
					Longitude:  geocode.Longitude,
					PersonID:   person.ID,
					PersonName: formatPersonName(person),
					EventDate:  person.BirthDate,
					Place:      person.BirthPlace,
				})
			}
		}
		
		// Death markers - skip unknown/placeholder places
		if person.DeathPlace != "" && !isUnknownPlace(person.DeathPlace) {
			geocode, err := s.GetPlaceGeocode(person.DeathPlace)
			if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
				markers = append(markers, &MapMarker{
					Type:       MarkerDeath,
					Latitude:   geocode.Latitude,
					Longitude:  geocode.Longitude,
					PersonID:   person.ID,
					PersonName: formatPersonName(person),
					EventDate:  person.DeathDate,
					Place:      person.DeathPlace,
				})
			}
		}

		// Life Events - generic one-off events (immigration, occupation, etc.)
		if events, err := s.GetEvents(person.ID); err == nil {
			for _, event := range events {
				if event.Place == "" || isUnknownPlace(event.Place) {
					continue
				}
				geocode, err := s.GetPlaceGeocode(event.Place)
				if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
					markers = append(markers, &MapMarker{
						Type:         MarkerEvent,
						Latitude:     geocode.Latitude,
						Longitude:    geocode.Longitude,
						PersonID:     person.ID,
						PersonName:   formatPersonName(person),
						EventDate:    event.Date,
						EventDateEnd: event.DateEnd,
						Place:        event.Place,
						EventLabel:   event.Type,
					})
				}
			}
		}
	}
	
	// Get marriage markers (from relationships)
	relationships, err := s.GetRelationships()
	if err != nil {
		return nil, err
	}
	
	// Marriage markers - skip unknown/placeholder places
	for _, rel := range relationships {
		if rel.Type == "spouse" && rel.MarriagePlace != "" && !isUnknownPlace(rel.MarriagePlace) {
			geocode, err := s.GetPlaceGeocode(rel.MarriagePlace)
			if err == nil && geocode != nil && geocode.GeocodeStatus == "success" {
				// Get person names
				person1, _ := s.GetPersonByID(rel.SubjectID)
				person2, _ := s.GetPersonByID(rel.ObjectID)
				
				if person1 != nil && person2 != nil {
					markers = append(markers, &MapMarker{
						Type:       MarkerMarriage,
						Latitude:   geocode.Latitude,
						Longitude:  geocode.Longitude,
						PersonID:   rel.SubjectID,
						PersonName: formatPersonName(*person1),
						SpouseID:   rel.ObjectID,
						SpouseName: formatPersonName(*person2),
						EventDate:  rel.MarriageDate,
						Place:      rel.MarriagePlace,
					})
				}
			}
		}
	}
	
	return markers, nil
}

// MarkerCluster groups nearby markers.
type MarkerCluster struct {
	Latitude  float64
	Longitude float64
	Markers   []*MapMarker
	Count     int
}

// ClusterMarkers groups markers that are close together.
func ClusterMarkers(markers []*MapMarker, zoomLevel int, threshold float64) []*MarkerCluster {
	if len(markers) == 0 {
		return []*MarkerCluster{}
	}
	
	// Adjust threshold based on zoom level (higher zoom = more precise clustering)
	adjustedThreshold := threshold / float64(zoomLevel)
	
	clusters := []*MarkerCluster{}
	used := make(map[int]bool)
	
	for i, marker := range markers {
		if used[i] {
			continue
		}
		
		// Create new cluster
		cluster := &MarkerCluster{
			Latitude:  marker.Latitude,
			Longitude: marker.Longitude,
			Markers:   []*MapMarker{marker},
			Count:     1,
		}
		used[i] = true
		
		// Find nearby markers
		for j, other := range markers {
			if i == j || used[j] {
				continue
			}
			
			// Calculate distance
			distance := CalculateDistance(marker.Latitude, marker.Longitude, other.Latitude, other.Longitude)
			
			if distance < adjustedThreshold {
				cluster.Markers = append(cluster.Markers, other)
				cluster.Count++
				// Update cluster center (average position)
				cluster.Latitude = (cluster.Latitude*float64(cluster.Count-1) + other.Latitude) / float64(cluster.Count)
				cluster.Longitude = (cluster.Longitude*float64(cluster.Count-1) + other.Longitude) / float64(cluster.Count)
				used[j] = true
			}
		}
		
		clusters = append(clusters, cluster)
	}
	
	return clusters
}

// CreateClusterWidget creates a widget for a marker cluster.
func CreateClusterWidget(cluster *MarkerCluster, onClick func()) fyne.CanvasObject {
	// Create cluster background circle
	circle := canvas.NewCircle(color.RGBA{R: 100, G: 150, B: 200, A: 200})
	circle.Resize(fyne.NewSize(30, 30))
	
	// Create count label
	countLabel := widget.NewLabel(fmt.Sprintf("%d", cluster.Count))
	
	// Create clickable button
	clusterBtn := widget.NewButton("", onClick)
	clusterBtn.Importance = widget.LowImportance
	
	// Stack layers
	return container.NewStack(
		circle,
		clusterBtn,
		container.NewCenter(countLabel),
	)
}

// GetMarkersForDescendants returns markers for all descendants of a person (Phase 5).
func GetMarkersForDescendants(s *store.Store, personID int64) ([]*MapMarker, error) {
	// Get all descendants recursively
	descendantIDs := getDescendantIDs(s, personID, make(map[int64]bool))
	
	// Include the person themselves
	descendantIDs[personID] = true
	
	// Collect markers for all descendants
	markers := []*MapMarker{}
	for id := range descendantIDs {
		personMarkers, err := GetMarkersForPerson(s, id)
		if err == nil {
			markers = append(markers, personMarkers...)
		}
	}
	
	return markers, nil
}

// GetMarkersForAncestors returns markers for all ancestors of a person (Phase 5).
func GetMarkersForAncestors(s *store.Store, personID int64) ([]*MapMarker, error) {
	// Get all ancestors recursively
	ancestorIDs := getAncestorIDs(s, personID, make(map[int64]bool))
	
	// Include the person themselves
	ancestorIDs[personID] = true
	
	// Collect markers for all ancestors
	markers := []*MapMarker{}
	for id := range ancestorIDs {
		personMarkers, err := GetMarkersForPerson(s, id)
		if err == nil {
			markers = append(markers, personMarkers...)
		}
	}
	
	return markers, nil
}

// getDescendantIDs recursively collects all descendant person IDs.
func getDescendantIDs(s *store.Store, personID int64, visited map[int64]bool) map[int64]bool {
	if visited[personID] {
		return visited
	}
	visited[personID] = true
	
	// Get all children
	children, err := s.GetRelatedPeople(personID, "child")
	if err != nil {
		return visited
	}
	
	// Recursively get descendants of each child
	for _, child := range children {
		getDescendantIDs(s, child.ID, visited)
	}
	
	return visited
}

// getAncestorIDs recursively collects all ancestor person IDs.
func getAncestorIDs(s *store.Store, personID int64, visited map[int64]bool) map[int64]bool {
	if visited[personID] {
		return visited
	}
	visited[personID] = true
	
	// Get person's parents
	parents, err := s.GetRelatedPeople(personID, "parent")
	if err != nil {
		return visited
	}
	
	// Recursively get ancestors of each parent
	for _, parent := range parents {
		getAncestorIDs(s, parent.ID, visited)
	}
	
	return visited
}

// CalculateGenerations calculates the generation for each marker relative to a reference person (Phase 5).
func CalculateGenerations(s *store.Store, markers []*MapMarker, refPersonID int64) {
	// Build generation map for all people
	generationMap := make(map[int64]int)
	generationMap[refPersonID] = 0
	
	// Calculate generations for descendants (positive numbers)
	calculateDescendantGenerations(s, refPersonID, 0, generationMap)
	
	// Calculate generations for ancestors (negative numbers)
	calculateAncestorGenerations(s, refPersonID, 0, generationMap)
	
	// Assign generations to markers
	for _, marker := range markers {
		if gen, ok := generationMap[marker.PersonID]; ok {
			marker.Generation = gen
		}
	}
}

// calculateDescendantGenerations recursively assigns generation numbers to descendants.
func calculateDescendantGenerations(s *store.Store, personID int64, currentGen int, generationMap map[int64]int) {
	children, err := s.GetRelatedPeople(personID, "child")
	if err != nil {
		return
	}
	
	for _, child := range children {
		if _, exists := generationMap[child.ID]; !exists {
			generationMap[child.ID] = currentGen + 1
			calculateDescendantGenerations(s, child.ID, currentGen+1, generationMap)
		}
	}
}

// calculateAncestorGenerations recursively assigns generation numbers to ancestors.
func calculateAncestorGenerations(s *store.Store, personID int64, currentGen int, generationMap map[int64]int) {
	parents, err := s.GetRelatedPeople(personID, "parent")
	if err != nil {
		return
	}
	
	// Process each parent
	for _, parent := range parents {
		if _, exists := generationMap[parent.ID]; !exists {
			generationMap[parent.ID] = currentGen - 1
			calculateAncestorGenerations(s, parent.ID, currentGen-1, generationMap)
		}
	}
}

// GetGenerationColor returns a color for a generation number (Phase 5).
// Uses a gradient from blue (ancestors) through green (current) to red (descendants).
func GetGenerationColor(generation int) color.Color {
	// Clamp generation to reasonable range (-5 to +5)
	if generation < -5 {
		generation = -5
	}
	if generation > 5 {
		generation = 5
	}
	
	// Map generation to color gradient
	// -5 to 0: blue to green
	// 0 to +5: green to red
	if generation < 0 {
		// Ancestors: blue to green
		ratio := float64(generation+5) / 5.0
		return color.RGBA{
			R: uint8(50 * ratio),
			G: uint8(100 + 100*ratio),
			B: uint8(255 - 155*ratio),
			A: 255,
		}
	} else if generation == 0 {
		// Current generation: bright green
		return color.RGBA{R: 50, G: 200, B: 50, A: 255}
	} else {
		// Descendants: green to red
		ratio := float64(generation) / 5.0
		return color.RGBA{
			R: uint8(50 + 205*ratio),
			G: uint8(200 - 150*ratio),
			B: uint8(50 * (1 - ratio)),
			A: 255,
		}
	}
}

// isUnknownPlace checks if a place name is a placeholder for unknown location.
func isUnknownPlace(place string) bool {
	lowerPlace := strings.ToLower(strings.TrimSpace(place))
	unknownPhrases := []string{
		"unknown",
		"not known",
		"?",
		"n/a",
		"na",
		"none",
		"--",
		"___",
		"tbd",
		"to be determined",
	}
	
	for _, phrase := range unknownPhrases {
		if lowerPlace == phrase {
			return true
		}
	}
	
	return false
}
