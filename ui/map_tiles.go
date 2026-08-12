package ui

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"sync"
	"time"

	"fyne.io/fyne/v2/canvas"
)

// TileCache caches downloaded map tiles in memory.
type TileCache struct {
	tiles map[string]*canvas.Image
	mu    sync.RWMutex
}

// NewTileCache creates a new tile cache.
func NewTileCache() *TileCache {
	return &TileCache{
		tiles: make(map[string]*canvas.Image),
	}
}

// Get retrieves a tile from cache.
func (tc *TileCache) Get(key string) (*canvas.Image, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	tile, ok := tc.tiles[key]
	return tile, ok
}

// Set stores a tile in cache.
func (tc *TileCache) Set(key string, tile *canvas.Image) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.tiles[key] = tile
}

// Clear removes all tiles from cache.
func (tc *TileCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.tiles = make(map[string]*canvas.Image)
}

// Size returns the number of cached tiles.
func (tc *TileCache) Size() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.tiles)
}

// TileDownloader downloads map tiles from OpenStreetMap.
type TileDownloader struct {
	client *http.Client
	cache  *TileCache
}

// NewTileDownloader creates a new tile downloader.
func NewTileDownloader() *TileDownloader {
	return &TileDownloader{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: NewTileCache(),
	}
}

// GetTile downloads or retrieves from cache a map tile.
func (td *TileDownloader) GetTile(x, y, zoom int) (*canvas.Image, error) {
	// Generate cache key
	key := fmt.Sprintf("%d-%d-%d", zoom, x, y)
	
	// Check cache first
	if tile, ok := td.cache.Get(key); ok {
		return tile, nil
	}
	
	// Download tile
	tileURL := GetTileURL(x, y, zoom)
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	
	// Set User-Agent (OSM tile usage policy)
	req.Header.Set("User-Agent", "KrankyBearGenealogy/1.5.0 (Genealogy Research Tool)")
	
	resp, err := td.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download tile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tile server returned status %d", resp.StatusCode)
	}
	
	// Read image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tile data: %w", err)
	}
	
	// Decode image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode tile image: %w", err)
	}
	
	// Create Fyne canvas image
	canvasImg := canvas.NewImageFromImage(img)
	canvasImg.FillMode = canvas.ImageFillOriginal
	
	// Cache it
	td.cache.Set(key, canvasImg)
	
	return canvasImg, nil
}

// DownloadTiles downloads multiple tiles concurrently.
func (td *TileDownloader) DownloadTiles(tiles []MapTile, maxConcurrent int) (map[string]*canvas.Image, error) {
	results := make(map[string]*canvas.Image)
	var mu sync.Mutex
	var failCount int
	
	// Use semaphore to limit concurrent downloads
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	
	for _, tile := range tiles {
		wg.Add(1)
		go func(t MapTile) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Download tile
			img, err := td.GetTile(t.X, t.Y, t.Zoom)
			if err != nil {
				// Track failures
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}
			
			// Store result
			key := fmt.Sprintf("%d-%d-%d", t.Zoom, t.X, t.Y)
			mu.Lock()
			results[key] = img
			mu.Unlock()
		}(tile)
	}
	
	wg.Wait()
	
	// If all tiles failed, likely no internet connection
	if failCount == len(tiles) && len(tiles) > 0 {
		return results, fmt.Errorf("all tiles failed to download - check internet connection")
	}
	
	return results, nil
}

// ClearCache clears the tile cache.
func (td *TileDownloader) ClearCache() {
	td.cache.Clear()
}

// CacheSize returns the number of cached tiles.
func (td *TileDownloader) CacheSize() int {
	return td.cache.Size()
}
