package ui

import (
	"fmt"
	"image/color"

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
)

// MapMarker represents a marker on the map for a life event.
type MapMarker struct {
	Type      MarkerType
	Latitude  float64
	Longitude float64
	PersonID  int64
	PersonName string
	EventDate string
	Place     string
	
	// Additional info for marriages
	SpouseID   int64
	SpouseName string
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
	default:
		return MarkerStyle{
			Color:  color.RGBA{R: 100, G: 100, B: 100, A: 255},
			Symbol: "📍",
			Size:   20,
		}
	}
}

// CreateMarkerWidget creates a clickable marker widget.
func CreateMarkerWidget(marker *MapMarker, onClick func()) fyne.CanvasObject {
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
	
	// Birth marker
	if person.BirthPlace != "" {
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
	
	// Death marker
	if person.DeathPlace != "" {
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
	
	// Collect birth markers
	for _, person := range people {
		if person.BirthPlace != "" {
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
		
		// Death markers
		if person.DeathPlace != "" {
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
	}
	
	// Get marriage markers (from relationships)
	relationships, err := s.GetRelationships()
	if err != nil {
		return nil, err
	}
	
	for _, rel := range relationships {
		if rel.Type == "spouse" && rel.MarriagePlace != "" {
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
