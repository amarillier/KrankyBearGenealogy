package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PlaceGeocode represents a geocoded place with coordinates.
type PlaceGeocode struct {
	ID            int64
	PlaceName     string
	Latitude      float64
	Longitude     float64
	Country       string
	StateProvince string
	City          string
	GeocodeDate   time.Time
	GeocodeStatus string // 'success', 'failed', 'ambiguous', 'manual', 'pending'
	Notes         string
}

// NominatimResult represents a result from the Nominatim API.
type NominatimResult struct {
	PlaceID     int64   `json:"place_id"`
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Type        string  `json:"type"`
	Address     Address `json:"address"`
}

// Address represents the address components from Nominatim.
type Address struct {
	City          string `json:"city"`
	Town          string `json:"town"`
	Village       string `json:"village"`
	County        string `json:"county"`
	State         string `json:"state"`
	Country       string `json:"country"`
	CountryCode   string `json:"country_code"`
	StateDistrict string `json:"state_district"`
}

// GetPlaceGeocode retrieves a cached geocode result.
func (s *Store) GetPlaceGeocode(placeName string) (*PlaceGeocode, error) {
	query := `SELECT id, place_name, latitude, longitude, country, state_province, city, 
              geocode_date, geocode_status, notes 
              FROM place_geocodes WHERE place_name = ?`
	
	var pg PlaceGeocode
	var geocodeDate string
	
	err := s.DB.QueryRow(query, placeName).Scan(
		&pg.ID, &pg.PlaceName, &pg.Latitude, &pg.Longitude,
		&pg.Country, &pg.StateProvince, &pg.City,
		&geocodeDate, &pg.GeocodeStatus, &pg.Notes,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil // Not found, not an error
	}
	if err != nil {
		return nil, err
	}
	
	pg.GeocodeDate, _ = time.Parse("2006-01-02 15:04:05", geocodeDate)
	return &pg, nil
}

// SavePlaceGeocode saves a geocode result to the cache.
func (s *Store) SavePlaceGeocode(pg *PlaceGeocode) error {
	query := `INSERT INTO place_geocodes 
              (place_name, latitude, longitude, country, state_province, city, geocode_status, notes)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT(place_name) DO UPDATE SET
              latitude = excluded.latitude,
              longitude = excluded.longitude,
              country = excluded.country,
              state_province = excluded.state_province,
              city = excluded.city,
              geocode_status = excluded.geocode_status,
              notes = excluded.notes,
              geocode_date = CURRENT_TIMESTAMP`
	
	_, err := s.DB.Exec(query, pg.PlaceName, pg.Latitude, pg.Longitude,
		pg.Country, pg.StateProvince, pg.City, pg.GeocodeStatus, pg.Notes)
	return err
}

// GeocodePlace geocodes a place name using Nominatim API with caching.
func (s *Store) GeocodePlace(placeName string) (*PlaceGeocode, error) {
	if placeName == "" {
		return nil, fmt.Errorf("empty place name")
	}
	
	// Check cache first
	cached, err := s.GetPlaceGeocode(placeName)
	if err != nil {
		return nil, err
	}
	if cached != nil && cached.GeocodeStatus == "success" {
		return cached, nil
	}
	
	// If we have a cached failure, check if there's now an alternate place mapping
	// (user may have added it after the initial failure)
	if cached != nil && cached.GeocodeStatus == "failed" {
		if altPlace, err := s.GetAlternatePlaceFor(placeName); err == nil && altPlace != nil {
			// Try geocoding the modern name
			time.Sleep(1 * time.Second) // Rate limit
			results, err := geocodeNominatim(altPlace.CurrentName)
			if err == nil && len(results) > 0 {
				// Success with alternate name! Use it
				result := results[0]
				lat, lon, err := parseLatLon(result.Lat, result.Lon)
				if err != nil {
					return nil, err
				}
				
				// Determine city name
				city := result.Address.City
				if city == "" {
					city = result.Address.Town
				}
				if city == "" {
					city = result.Address.Village
				}
				
				pg := &PlaceGeocode{
					PlaceName:     placeName, // Store original historical name
					Latitude:      lat,
					Longitude:     lon,
					Country:       result.Address.Country,
					StateProvince: result.Address.State,
					City:          city,
					GeocodeStatus: "success",
					Notes:         fmt.Sprintf("Geocoded via modern name: %s → %s", placeName, altPlace.CurrentName),
				}
				
				// Save to cache (updates the failed entry)
				if err := s.SavePlaceGeocode(pg); err != nil {
					return nil, err
				}
				
				return pg, nil
			}
		}
		// Still failed, return cached failure
		return cached, fmt.Errorf("no results found for: %s", placeName)
	}
	
	// Rate limit: Nominatim requires 1 request per second
	time.Sleep(1 * time.Second)
	
	// Call Nominatim API
	results, err := geocodeNominatim(placeName)
	if err != nil {
		// Save failed result
		pg := &PlaceGeocode{
			PlaceName:     placeName,
			GeocodeStatus: "failed",
			Notes:         err.Error(),
		}
		s.SavePlaceGeocode(pg)
		return nil, err
	}
	
	if len(results) == 0 {
		// No results found - try alternate place name if available
		if altPlace, err := s.GetAlternatePlaceFor(placeName); err == nil && altPlace != nil {
			// Try geocoding the current/modern name
			time.Sleep(1 * time.Second) // Rate limit
			results, err = geocodeNominatim(altPlace.CurrentName)
			if err == nil && len(results) > 0 {
				// Success with alternate name! Use it
				result := results[0]
				lat, lon, err := parseLatLon(result.Lat, result.Lon)
				if err != nil {
					return nil, err
				}
				
				// Determine city name
				city := result.Address.City
				if city == "" {
					city = result.Address.Town
				}
				if city == "" {
					city = result.Address.Village
				}
				
				pg := &PlaceGeocode{
					PlaceName:     placeName, // Store original historical name
					Latitude:      lat,
					Longitude:     lon,
					Country:       result.Address.Country,
					StateProvince: result.Address.State,
					City:          city,
					GeocodeStatus: "success",
					Notes:         fmt.Sprintf("Geocoded via modern name: %s → %s", placeName, altPlace.CurrentName),
				}
				
				// Save to cache
				if err := s.SavePlaceGeocode(pg); err != nil {
					return nil, err
				}
				
				return pg, nil
			}
		}
		
		// Still no results found
		pg := &PlaceGeocode{
			PlaceName:     placeName,
			GeocodeStatus: "failed",
			Notes:         "No results found",
		}
		s.SavePlaceGeocode(pg)
		return nil, fmt.Errorf("no results found for: %s", placeName)
	}
	
	// Use first result (most relevant)
	result := results[0]
	lat, lon, err := parseLatLon(result.Lat, result.Lon)
	if err != nil {
		return nil, err
	}
	
	// Determine city name (try city, town, or village)
	city := result.Address.City
	if city == "" {
		city = result.Address.Town
	}
	if city == "" {
		city = result.Address.Village
	}
	
	pg := &PlaceGeocode{
		PlaceName:     placeName,
		Latitude:      lat,
		Longitude:     lon,
		Country:       result.Address.Country,
		StateProvince: result.Address.State,
		City:          city,
		GeocodeStatus: "success",
		Notes:         result.DisplayName,
	}
	
	// Save to cache
	if err := s.SavePlaceGeocode(pg); err != nil {
		return nil, err
	}
	
	return pg, nil
}

// geocodeNominatim calls the Nominatim API to geocode a place.
func geocodeNominatim(placeName string) ([]NominatimResult, error) {
	baseURL := "https://nominatim.openstreetmap.org/search"
	
	params := url.Values{}
	params.Add("q", placeName)
	params.Add("format", "json")
	params.Add("limit", "5")
	params.Add("addressdetails", "1")
	
	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	
	// Create request with User-Agent (required by Nominatim)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "KrankyBearGenealogy/1.5.0 (Genealogy Research Tool)")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nominatim returned status %d: %s", resp.StatusCode, string(body))
	}
	
	var results []NominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to parse nominatim response: %w", err)
	}
	
	return results, nil
}

// parseLatLon converts string coordinates to float64.
func parseLatLon(latStr, lonStr string) (float64, float64, error) {
	var lat, lon float64
	if _, err := fmt.Sscanf(latStr, "%f", &lat); err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %s", latStr)
	}
	if _, err := fmt.Sscanf(lonStr, "%f", &lon); err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %s", lonStr)
	}
	return lat, lon, nil
}

// GetAllUniquePlaces returns all unique place names from the database.
func (s *Store) GetAllUniquePlaces() ([]string, error) {
	query := `
		SELECT DISTINCT place FROM (
			SELECT birth_place as place FROM persons WHERE birth_place != ''
			UNION
			SELECT death_place as place FROM persons WHERE death_place != ''
		) WHERE place IS NOT NULL
		ORDER BY place`
	
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var places []string
	for rows.Next() {
		var place string
		if err := rows.Scan(&place); err != nil {
			return nil, err
		}
		places = append(places, place)
	}
	
	return places, rows.Err()
}

// GetUnmappedPlaces returns places that haven't been geocoded or failed.
func (s *Store) GetUnmappedPlaces() ([]string, error) {
	allPlaces, err := s.GetAllUniquePlaces()
	if err != nil {
		return nil, err
	}
	
	var unmapped []string
	for _, place := range allPlaces {
		cached, err := s.GetPlaceGeocode(place)
		if err != nil {
			return nil, err
		}
		if cached == nil || cached.GeocodeStatus != "success" {
			unmapped = append(unmapped, place)
		}
	}
	
	return unmapped, nil
}

// GetGeocodedPlaces returns all successfully geocoded places.
func (s *Store) GetGeocodedPlaces() ([]*PlaceGeocode, error) {
	query := `SELECT id, place_name, latitude, longitude, country, state_province, city,
              geocode_date, geocode_status, notes 
              FROM place_geocodes 
              WHERE geocode_status = 'success' 
              ORDER BY place_name`
	
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var geocodes []*PlaceGeocode
	for rows.Next() {
		var pg PlaceGeocode
		var geocodeDate string
		
		err := rows.Scan(&pg.ID, &pg.PlaceName, &pg.Latitude, &pg.Longitude,
			&pg.Country, &pg.StateProvince, &pg.City,
			&geocodeDate, &pg.GeocodeStatus, &pg.Notes)
		if err != nil {
			return nil, err
		}
		
		pg.GeocodeDate, _ = time.Parse("2006-01-02 15:04:05", geocodeDate)
		geocodes = append(geocodes, &pg)
	}
	
	return geocodes, rows.Err()
}

// CleanPlaceName normalizes place names for better geocoding results.
func CleanPlaceName(place string) string {
	// Trim whitespace
	place = strings.TrimSpace(place)
	
	// Remove common genealogy annotations
	place = strings.ReplaceAll(place, "?", "")
	place = strings.ReplaceAll(place, "abt.", "")
	place = strings.ReplaceAll(place, "about", "")
	place = strings.ReplaceAll(place, "circa", "")
	
	// Collapse multiple spaces
	for strings.Contains(place, "  ") {
		place = strings.ReplaceAll(place, "  ", " ")
	}
	
	return strings.TrimSpace(place)
}
