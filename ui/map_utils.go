package ui

import (
	"fmt"
	"math"
)

// MapTile represents a single map tile with its coordinates.
type MapTile struct {
	X    int
	Y    int
	Zoom int
	URL  string
}

// LatLng represents a geographic coordinate.
type LatLng struct {
	Lat float64
	Lon float64
}

// TileCoord represents tile coordinates at a specific zoom level.
type TileCoord struct {
	X    int
	Y    int
	Zoom int
}

// MapBounds represents the geographic bounds of a region.
type MapBounds struct {
	North float64
	South float64
	East  float64
	West  float64
}

// LatLngToTile converts latitude/longitude to tile coordinates.
// OSM tile numbering: https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
func LatLngToTile(lat, lon float64, zoom int) TileCoord {
	n := math.Pow(2, float64(zoom))
	x := int((lon + 180.0) / 360.0 * n)
	
	latRad := lat * math.Pi / 180.0
	y := int((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n)
	
	return TileCoord{
		X:    x,
		Y:    y,
		Zoom: zoom,
	}
}

// TileToLatLng converts tile coordinates to the northwest corner lat/lng.
func TileToLatLng(x, y, zoom int) LatLng {
	n := math.Pow(2, float64(zoom))
	lon := float64(x)/n*360.0 - 180.0
	
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	lat := latRad * 180.0 / math.Pi
	
	return LatLng{Lat: lat, Lon: lon}
}

// GetTileURL generates the OpenStreetMap tile URL for given coordinates.
func GetTileURL(x, y, zoom int) string {
	// Use one of the OSM tile servers
	// Rotate between a, b, c servers for better load distribution
	server := "a"
	switch (x + y) % 3 {
	case 1:
		server = "b"
	case 2:
		server = "c"
	}
	
	return fmt.Sprintf("https://%s.tile.openstreetmap.org/%d/%d/%d.png", server, zoom, x, y)
}

// GetVisibleTiles calculates which tiles are needed to cover the viewport.
func GetVisibleTiles(centerLat, centerLon float64, zoom, viewportWidth, viewportHeight int) []MapTile {
	// Tile size in pixels (OSM standard)
	const tileSize = 256
	
	// Calculate how many tiles we need in each direction
	tilesX := (viewportWidth / tileSize) + 2  // +2 for partial tiles on edges
	tilesY := (viewportHeight / tileSize) + 2
	
	// Get center tile
	centerTile := LatLngToTile(centerLat, centerLon, zoom)
	
	// Calculate tile range
	minTileX := centerTile.X - tilesX/2
	maxTileX := centerTile.X + tilesX/2
	minTileY := centerTile.Y - tilesY/2
	maxTileY := centerTile.Y + tilesY/2
	
	// Clamp to valid tile range
	maxTiles := int(math.Pow(2, float64(zoom)))
	if minTileX < 0 {
		minTileX = 0
	}
	if maxTileX >= maxTiles {
		maxTileX = maxTiles - 1
	}
	if minTileY < 0 {
		minTileY = 0
	}
	if maxTileY >= maxTiles {
		maxTileY = maxTiles - 1
	}
	
	// Generate tile list
	tiles := []MapTile{}
	for x := minTileX; x <= maxTileX; x++ {
		for y := minTileY; y <= maxTileY; y++ {
			tiles = append(tiles, MapTile{
				X:    x,
				Y:    y,
				Zoom: zoom,
				URL:  GetTileURL(x, y, zoom),
			})
		}
	}
	
	return tiles
}

// CalculateDistance calculates the distance between two points in kilometers.
// Uses the Haversine formula.
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // Earth's radius in kilometers
	
	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	deltaLat := (lat2 - lat1) * math.Pi / 180.0
	deltaLon := (lon2 - lon1) * math.Pi / 180.0
	
	// Haversine formula
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	
	return earthRadius * c
}

// GetBoundsForPoints calculates the bounding box for a set of coordinates.
func GetBoundsForPoints(points []LatLng) MapBounds {
	if len(points) == 0 {
		return MapBounds{
			North: 90.0,
			South: -90.0,
			East:  180.0,
			West:  -180.0,
		}
	}
	
	bounds := MapBounds{
		North: points[0].Lat,
		South: points[0].Lat,
		East:  points[0].Lon,
		West:  points[0].Lon,
	}
	
	for _, p := range points[1:] {
		if p.Lat > bounds.North {
			bounds.North = p.Lat
		}
		if p.Lat < bounds.South {
			bounds.South = p.Lat
		}
		if p.Lon > bounds.East {
			bounds.East = p.Lon
		}
		if p.Lon < bounds.West {
			bounds.West = p.Lon
		}
	}
	
	return bounds
}

// GetCenterOfBounds calculates the center point of a bounding box.
func GetCenterOfBounds(bounds MapBounds) LatLng {
	return LatLng{
		Lat: (bounds.North + bounds.South) / 2,
		Lon: (bounds.East + bounds.West) / 2,
	}
}

// CalculateZoomForBounds determines the appropriate zoom level for a bounding box.
func CalculateZoomForBounds(bounds MapBounds, viewportWidth, viewportHeight int) int {
	// Tile size in pixels
	const tileSize = 256
	
	// Calculate required map size to fit bounds
	deltaLat := bounds.North - bounds.South
	deltaLon := bounds.East - bounds.West
	
	// Start with a conservative zoom level
	for zoom := 18; zoom >= 1; zoom-- {
		// Calculate world size at this zoom level
		worldSize := tileSize * math.Pow(2, float64(zoom))
		
		// Calculate how many pixels the bounds would occupy
		pixelsX := (deltaLon / 360.0) * worldSize
		pixelsY := (deltaLat / 180.0) * worldSize * 0.5 // Approximate for Mercator
		
		// If it fits in viewport, use this zoom
		if pixelsX <= float64(viewportWidth)*0.8 && pixelsY <= float64(viewportHeight)*0.8 {
			return zoom
		}
	}
	
	return 1 // Minimum zoom if nothing else fits
}
