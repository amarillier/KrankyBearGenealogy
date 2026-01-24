package store

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

// DatabaseStats contains statistics about the database
type DatabaseStats struct {
	FileSizeBytes     int64
	FileSizeMB        float64
	TotalPeople       int
	TotalRelationships int
	TotalMedia        int
	TotalSources      int
	TotalCitations    int
	TotalResearchLogs int
	TotalTodos        int
	OrphanedMedia     int
	OrphanedRelationships int
	BrokenRelationships int
	UnusedSources     int
	LastVacuumed      time.Time
}

// MaintenanceReport contains the results of a maintenance operation
type MaintenanceReport struct {
	Operation        string
	Success          bool
	Message          string
	ItemsProcessed   int
	ItemsRemoved     int
	SpaceReclaimed   int64
	Duration         time.Duration
	Details          []string
}

// GetDatabasePath returns the path to the database file
func (s *Store) GetDatabasePath() (string, error) {
	var path string
	err := s.DB.QueryRow("PRAGMA database_list").Scan(nil, nil, &path)
	if err != nil {
		return "", err
	}
	return path, nil
}

// GetDatabaseStats returns comprehensive statistics about the database
func (s *Store) GetDatabaseStats() (*DatabaseStats, error) {
	stats := &DatabaseStats{}
	
	// Get database file path and size
	rows, err := s.DB.Query("PRAGMA database_list")
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var seq int
			var name string
			var dbPath string
			if err := rows.Scan(&seq, &name, &dbPath); err == nil {
				if fileInfo, err := os.Stat(dbPath); err == nil {
					stats.FileSizeBytes = fileInfo.Size()
					stats.FileSizeMB = float64(fileInfo.Size()) / (1024 * 1024)
				}
			}
		}
	}
	
	// Count people
	err = s.DB.QueryRow("SELECT COUNT(*) FROM persons").Scan(&stats.TotalPeople)
	if err != nil {
		return nil, fmt.Errorf("count people: %w", err)
	}
	
	// Count relationships
	err = s.DB.QueryRow("SELECT COUNT(*) FROM relationships").Scan(&stats.TotalRelationships)
	if err != nil {
		return nil, fmt.Errorf("count relationships: %w", err)
	}
	
	// Count media
	err = s.DB.QueryRow("SELECT COUNT(*) FROM media").Scan(&stats.TotalMedia)
	if err != nil {
		return nil, fmt.Errorf("count media: %w", err)
	}
	
	// Count sources
	err = s.DB.QueryRow("SELECT COUNT(*) FROM sources").Scan(&stats.TotalSources)
	if err != nil {
		stats.TotalSources = 0 // Table might not exist in older versions
	}
	
	// Count citations
	err = s.DB.QueryRow("SELECT COUNT(*) FROM citations").Scan(&stats.TotalCitations)
	if err != nil {
		stats.TotalCitations = 0 // Table might not exist in older versions
	}
	
	// Count research logs
	err = s.DB.QueryRow("SELECT COUNT(*) FROM research_logs").Scan(&stats.TotalResearchLogs)
	if err != nil {
		stats.TotalResearchLogs = 0 // Table might not exist in older versions
	}
	
	// Count todos
	err = s.DB.QueryRow("SELECT COUNT(*) FROM todos").Scan(&stats.TotalTodos)
	if err != nil {
		stats.TotalTodos = 0 // Table might not exist in older versions
	}
	
	// Count orphaned media (media not linked to any person)
	err = s.DB.QueryRow(`
		SELECT COUNT(*) FROM media 
		WHERE id NOT IN (SELECT DISTINCT media_id FROM person_media)
	`).Scan(&stats.OrphanedMedia)
	if err != nil {
		stats.OrphanedMedia = 0
	}
	
	// Count orphaned relationships (relationships referencing non-existent people)
	err = s.DB.QueryRow(`
		SELECT COUNT(*) FROM relationships 
		WHERE subject_id NOT IN (SELECT id FROM persons) 
		   OR object_id NOT IN (SELECT id FROM persons)
	`).Scan(&stats.OrphanedRelationships)
	if err != nil {
		stats.OrphanedRelationships = 0
	}
	
	// Count broken relationships (duplicate or invalid)
	err = s.DB.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT subject_id, object_id, type 
			FROM relationships 
			GROUP BY subject_id, object_id, type 
			HAVING COUNT(*) > 1
		)
	`).Scan(&stats.BrokenRelationships)
	if err != nil {
		stats.BrokenRelationships = 0
	}
	
	// Count unused sources (sources not cited by anyone)
	err = s.DB.QueryRow(`
		SELECT COUNT(*) FROM sources 
		WHERE id NOT IN (SELECT DISTINCT source_id FROM citations)
	`).Scan(&stats.UnusedSources)
	if err != nil {
		stats.UnusedSources = 0
	}
	
	return stats, nil
}

// VacuumDatabase runs VACUUM to reclaim space and optimize the database
func (s *Store) VacuumDatabase() (*MaintenanceReport, error) {
	start := time.Now()
	report := &MaintenanceReport{
		Operation: "Vacuum Database",
		Details:   []string{},
	}
	
	// Get size before
	stats, err := s.GetDatabaseStats()
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to get database stats: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	sizeBefore := stats.FileSizeBytes
	
	// Run VACUUM
	_, err = s.DB.Exec("VACUUM")
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to vacuum database: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	
	// Get size after
	stats, err = s.GetDatabaseStats()
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to get database stats after vacuum: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	sizeAfter := stats.FileSizeBytes
	spaceReclaimed := sizeBefore - sizeAfter
	
	report.Success = true
	report.SpaceReclaimed = spaceReclaimed
	report.Duration = time.Since(start)
	
	if spaceReclaimed > 0 {
		report.Message = fmt.Sprintf("Database optimized successfully. Reclaimed %.2f MB", float64(spaceReclaimed)/(1024*1024))
		report.Details = append(report.Details, fmt.Sprintf("Size before: %.2f MB", float64(sizeBefore)/(1024*1024)))
		report.Details = append(report.Details, fmt.Sprintf("Size after: %.2f MB", float64(sizeAfter)/(1024*1024)))
	} else {
		report.Message = "Database optimized successfully. No additional space reclaimed."
	}
	
	return report, nil
}

// CheckIntegrity runs SQLite's integrity_check
func (s *Store) CheckIntegrity() (*MaintenanceReport, error) {
	start := time.Now()
	report := &MaintenanceReport{
		Operation: "Check Database Integrity",
		Details:   []string{},
	}
	
	rows, err := s.DB.Query("PRAGMA integrity_check")
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to check integrity: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	defer rows.Close()
	
	issues := []string{}
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			continue
		}
		if result != "ok" {
			issues = append(issues, result)
		}
	}
	
	report.Duration = time.Since(start)
	
	if len(issues) == 0 {
		report.Success = true
		report.Message = "Database integrity check passed. No issues found."
	} else {
		report.Success = false
		report.Message = fmt.Sprintf("Found %d integrity issue(s)", len(issues))
		report.Details = issues
	}
	
	return report, nil
}

// RemoveOrphanedMedia removes media records not linked to any person
func (s *Store) RemoveOrphanedMedia() (*MaintenanceReport, error) {
	start := time.Now()
	report := &MaintenanceReport{
		Operation: "Remove Orphaned Media",
		Details:   []string{},
	}
	
	// Find orphaned media
	rows, err := s.DB.Query(`
		SELECT id, title FROM media 
		WHERE id NOT IN (SELECT DISTINCT media_id FROM person_media)
	`)
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to query orphaned media: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	defer rows.Close()
	
	orphanedIDs := []int64{}
	orphanedNames := []string{}
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}
		orphanedIDs = append(orphanedIDs, id)
		if title == "" {
			title = fmt.Sprintf("Media #%d", id)
		}
		orphanedNames = append(orphanedNames, title)
	}
	
	if len(orphanedIDs) == 0 {
		report.Success = true
		report.Message = "No orphaned media found."
		report.Duration = time.Since(start)
		return report, nil
	}
	
	// Delete orphaned media
	result, err := s.DB.Exec(`
		DELETE FROM media 
		WHERE id NOT IN (SELECT DISTINCT media_id FROM person_media)
	`)
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to delete orphaned media: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	
	rowsAffected, _ := result.RowsAffected()
	report.Success = true
	report.ItemsRemoved = int(rowsAffected)
	report.Message = fmt.Sprintf("Removed %d orphaned media record(s)", rowsAffected)
	report.Duration = time.Since(start)
	
	// Add details
	for i, name := range orphanedNames {
		if i < 10 { // Limit to first 10
			report.Details = append(report.Details, name)
		}
	}
	if len(orphanedNames) > 10 {
		report.Details = append(report.Details, fmt.Sprintf("... and %d more", len(orphanedNames)-10))
	}
	
	return report, nil
}

// RemoveOrphanedRelationships removes relationships referencing non-existent people
func (s *Store) RemoveOrphanedRelationships() (*MaintenanceReport, error) {
	start := time.Now()
	report := &MaintenanceReport{
		Operation: "Remove Orphaned Relationships",
		Details:   []string{},
	}
	
	// Find orphaned relationships
	rows, err := s.DB.Query(`
		SELECT id, type, subject_id, object_id FROM relationships 
		WHERE subject_id NOT IN (SELECT id FROM persons) 
		   OR object_id NOT IN (SELECT id FROM persons)
	`)
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to query orphaned relationships: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	defer rows.Close()
	
	orphanedCount := 0
	for rows.Next() {
		var id, subjectID, objectID int64
		var relType string
		if err := rows.Scan(&id, &relType, &subjectID, &objectID); err != nil {
			continue
		}
		orphanedCount++
		if orphanedCount <= 10 {
			report.Details = append(report.Details, fmt.Sprintf("%s: %d → %d", relType, subjectID, objectID))
		}
	}
	
	if orphanedCount == 0 {
		report.Success = true
		report.Message = "No orphaned relationships found."
		report.Duration = time.Since(start)
		return report, nil
	}
	
	// Delete orphaned relationships
	result, err := s.DB.Exec(`
		DELETE FROM relationships 
		WHERE subject_id NOT IN (SELECT id FROM persons) 
		   OR object_id NOT IN (SELECT id FROM persons)
	`)
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to delete orphaned relationships: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	
	rowsAffected, _ := result.RowsAffected()
	report.Success = true
	report.ItemsRemoved = int(rowsAffected)
	report.Message = fmt.Sprintf("Removed %d orphaned relationship(s)", rowsAffected)
	report.Duration = time.Since(start)
	
	if orphanedCount > 10 {
		report.Details = append(report.Details, fmt.Sprintf("... and %d more", orphanedCount-10))
	}
	
	return report, nil
}

// RemoveDuplicateRelationships removes duplicate relationship entries
func (s *Store) RemoveDuplicateRelationships() (*MaintenanceReport, error) {
	start := time.Now()
	report := &MaintenanceReport{
		Operation: "Remove Duplicate Relationships",
		Details:   []string{},
	}
	
	// Find and keep only the oldest relationship for each duplicate set
	result, err := s.DB.Exec(`
		DELETE FROM relationships 
		WHERE id NOT IN (
			SELECT MIN(id) 
			FROM relationships 
			GROUP BY subject_id, object_id, type
		)
	`)
	if err != nil {
		report.Success = false
		report.Message = fmt.Sprintf("Failed to remove duplicates: %v", err)
		report.Duration = time.Since(start)
		return report, err
	}
	
	rowsAffected, _ := result.RowsAffected()
	report.Success = true
	report.ItemsRemoved = int(rowsAffected)
	report.Duration = time.Since(start)
	
	if rowsAffected > 0 {
		report.Message = fmt.Sprintf("Removed %d duplicate relationship(s)", rowsAffected)
	} else {
		report.Message = "No duplicate relationships found."
	}
	
	return report, nil
}

// AnalyzeDatabase runs ANALYZE to update query planner statistics
func (s *Store) AnalyzeDatabase() error {
	_, err := s.DB.Exec("ANALYZE")
	return err
}

// GetUnusedSources returns sources that have no citations
func (s *Store) GetUnusedSources() ([]Source, error) {
	rows, err := s.DB.Query(`
		SELECT id, title, author, publication, repository, 
		       call_number, source_type, notes, created_at, updated_at
		FROM sources 
		WHERE id NOT IN (SELECT DISTINCT source_id FROM citations)
		ORDER BY title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var sources []Source
	for rows.Next() {
		var src Source
		var createdAt, updatedAt sql.NullString
		err := rows.Scan(&src.ID, &src.Title, &src.Author, &src.Publication, 
			&src.Repository, &src.CallNumber, &src.SourceType, &src.Notes,
			&createdAt, &updatedAt)
		if err != nil {
			continue
		}
		sources = append(sources, src)
	}
	
	return sources, nil
}
