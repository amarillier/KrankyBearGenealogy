package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// showDatabaseMaintenanceDialog displays the database maintenance and analysis window
func showDatabaseMaintenanceDialog(w fyne.Window, s *store.Store) {
	// Create a new window instead of a modal dialog for better UX
	maintenanceWindow := fyne.CurrentApp().NewWindow("Database Maintenance & Analysis")
	
	// Create tabs for different maintenance operations
	tabs := container.NewAppTabs(
		container.NewTabItem("Statistics", createStatisticsTab(maintenanceWindow, s)),
		container.NewTabItem("Maintenance", createMaintenanceTab(maintenanceWindow, s)),
		container.NewTabItem("Analysis", createAnalysisTab(maintenanceWindow, s)),
	)
	
	tabs.SetTabLocation(container.TabLocationTop)
	
	maintenanceWindow.SetContent(tabs)
	maintenanceWindow.Resize(fyne.NewSize(900, 700))
	maintenanceWindow.Show()
}

// createStatisticsTab creates the database statistics display tab
func createStatisticsTab(w fyne.Window, s *store.Store) fyne.CanvasObject {
	statsText := widget.NewLabel("Loading statistics...")
	statsText.Wrapping = fyne.TextWrapWord
	statsText.TextStyle.Monospace = true // Use monospace for better formatted output
	
	var refreshBtn *widget.Button
	refreshBtn = widget.NewButton("Refresh Statistics", func() {
		refreshBtn.Disable()
		statsText.SetText("Loading statistics...")
		go func() {
			updateStatistics(statsText, s)
			fyne.Do(func() {
				refreshBtn.Enable()
			})
		}()
	})
	
	// Initial load in background
	go func() {
		updateStatistics(statsText, s)
	}()
	
	return container.NewBorder(
		nil,
		refreshBtn,
		nil, nil,
		container.NewScroll(statsText),
	)
}

// updateStatistics fetches and displays current database statistics
func updateStatistics(label *widget.Label, s *store.Store) {
	stats, err := s.GetDatabaseStats()
	if err != nil {
		fyne.Do(func() {
			label.SetText(fmt.Sprintf("Error loading statistics: %v", err))
		})
		return
	}
	
	var sb strings.Builder
	sb.WriteString("📊 DATABASE STATISTICS\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	
	sb.WriteString("📁 Database File:\n")
	sb.WriteString(fmt.Sprintf("   Size: %.2f MB (%d bytes)\n\n", stats.FileSizeMB, stats.FileSizeBytes))
	
	sb.WriteString("👥 Records:\n")
	sb.WriteString(fmt.Sprintf("   People: %d\n", stats.TotalPeople))
	sb.WriteString(fmt.Sprintf("   Relationships: %d\n", stats.TotalRelationships))
	sb.WriteString(fmt.Sprintf("   Media Files: %d\n", stats.TotalMedia))
	sb.WriteString(fmt.Sprintf("   Sources: %d\n", stats.TotalSources))
	sb.WriteString(fmt.Sprintf("   Citations: %d\n", stats.TotalCitations))
	sb.WriteString(fmt.Sprintf("   Research Logs: %d\n", stats.TotalResearchLogs))
	sb.WriteString(fmt.Sprintf("   To-Do Items: %d\n\n", stats.TotalTodos))
	
	// Data quality issues
	hasIssues := false
	if stats.OrphanedMedia > 0 || stats.OrphanedRelationships > 0 || 
	   stats.BrokenRelationships > 0 || stats.UnusedSources > 0 {
		hasIssues = true
		sb.WriteString("⚠️  Data Quality Issues:\n")
		if stats.OrphanedMedia > 0 {
			sb.WriteString(fmt.Sprintf("   Orphaned Media: %d (not linked to any person)\n", stats.OrphanedMedia))
		}
		if stats.OrphanedRelationships > 0 {
			sb.WriteString(fmt.Sprintf("   Orphaned Relationships: %d (reference deleted people)\n", stats.OrphanedRelationships))
		}
		if stats.BrokenRelationships > 0 {
			sb.WriteString(fmt.Sprintf("   Duplicate Relationships: %d\n", stats.BrokenRelationships))
		}
		if stats.UnusedSources > 0 {
			sb.WriteString(fmt.Sprintf("   Unused Sources: %d (no citations)\n", stats.UnusedSources))
		}
		sb.WriteString("\n")
	}
	
	if !hasIssues {
		sb.WriteString("✅ Data Quality: No issues found!\n\n")
	} else {
		sb.WriteString("💡 Use the Maintenance tab to fix these issues.\n\n")
	}
	
	fyne.Do(func() {
		label.SetText(sb.String())
	})
}

// createMaintenanceTab creates the maintenance operations tab
func createMaintenanceTab(w fyne.Window, s *store.Store) fyne.CanvasObject {
	resultText := widget.NewLabel("Select a maintenance operation to begin.")
	resultText.Wrapping = fyne.TextWrapWord
	resultText.TextStyle.Monospace = true // Use monospace for better formatted output
	
	// Vacuum button
	var vacuumBtn *widget.Button
	vacuumBtn = widget.NewButton("🧹 Vacuum Database", func() {
		vacuumBtn.Disable()
		resultText.SetText("Running VACUUM... This may take a moment...")
		
		go func() {
			defer fyne.Do(func() {
				vacuumBtn.Enable()
			})
			
			report, err := s.VacuumDatabase()
			if err != nil {
				fyne.Do(func() {
					resultText.SetText(fmt.Sprintf("Error: %v", err))
				})
				return
			}
			
			fyne.Do(func() {
				displayMaintenanceReport(resultText, report)
			})
		}()
	})
	vacuumBtn.Importance = widget.HighImportance
	
	vacuumDesc := widget.NewLabel("Optimizes the database and reclaims unused space. Recommended after deleting records.")
	vacuumDesc.Wrapping = fyne.TextWrapWord
	vacuumDesc.TextStyle.Italic = true
	
	// Integrity check button
	var integrityBtn *widget.Button
	integrityBtn = widget.NewButton("🔍 Check Integrity", func() {
		integrityBtn.Disable()
		resultText.SetText("Checking database integrity...")
		
		go func() {
			defer fyne.Do(func() {
				integrityBtn.Enable()
			})
			
			report, err := s.CheckIntegrity()
			if err != nil {
				fyne.Do(func() {
					resultText.SetText(fmt.Sprintf("Error: %v", err))
				})
				return
			}
			
			fyne.Do(func() {
				displayMaintenanceReport(resultText, report)
			})
		}()
	})
	
	integrityDesc := widget.NewLabel("Verifies that the database file is not corrupted.")
	integrityDesc.Wrapping = fyne.TextWrapWord
	integrityDesc.TextStyle.Italic = true
	
	// Remove orphaned media button
	var orphanedMediaBtn *widget.Button
	orphanedMediaBtn = widget.NewButton("🗑️ Remove Orphaned Media", func() {
		dialog.ShowConfirm("Remove Orphaned Media",
			"This will permanently delete media records that are not linked to any person.\n\nContinue?",
			func(ok bool) {
				if !ok {
					return
				}
				
				orphanedMediaBtn.Disable()
				resultText.SetText("Removing orphaned media...")
				
				go func() {
					defer fyne.Do(func() {
						orphanedMediaBtn.Enable()
					})
					
					report, err := s.RemoveOrphanedMedia()
					if err != nil {
						fyne.Do(func() {
							resultText.SetText(fmt.Sprintf("Error: %v", err))
						})
						return
					}
					
					fyne.Do(func() {
						displayMaintenanceReport(resultText, report)
					})
				}()
			}, w)
	})
	orphanedMediaBtn.Importance = widget.WarningImportance
	
	orphanedMediaDesc := widget.NewLabel("Removes media files not linked to any person.")
	orphanedMediaDesc.Wrapping = fyne.TextWrapWord
	orphanedMediaDesc.TextStyle.Italic = true
	
	// Remove orphaned relationships button
	var orphanedRelBtn *widget.Button
	orphanedRelBtn = widget.NewButton("🗑️ Remove Orphaned Relationships", func() {
		dialog.ShowConfirm("Remove Orphaned Relationships",
			"This will permanently delete relationships that reference non-existent people.\n\nContinue?",
			func(ok bool) {
				if !ok {
					return
				}
				
				orphanedRelBtn.Disable()
				resultText.SetText("Removing orphaned relationships...")
				
				go func() {
					defer fyne.Do(func() {
						orphanedRelBtn.Enable()
					})
					
					report, err := s.RemoveOrphanedRelationships()
					if err != nil {
						fyne.Do(func() {
							resultText.SetText(fmt.Sprintf("Error: %v", err))
						})
						return
					}
					
					fyne.Do(func() {
						displayMaintenanceReport(resultText, report)
					})
				}()
			}, w)
	})
	orphanedRelBtn.Importance = widget.WarningImportance
	
	orphanedRelDesc := widget.NewLabel("Removes relationships referencing deleted people.")
	orphanedRelDesc.Wrapping = fyne.TextWrapWord
	orphanedRelDesc.TextStyle.Italic = true
	
	// Remove duplicate relationships button
	var duplicateRelBtn *widget.Button
	duplicateRelBtn = widget.NewButton("🗑️ Remove Duplicate Relationships", func() {
		dialog.ShowConfirm("Remove Duplicate Relationships",
			"This will remove duplicate relationship entries, keeping only the oldest one.\n\nContinue?",
			func(ok bool) {
				if !ok {
					return
				}
				
				duplicateRelBtn.Disable()
				resultText.SetText("Removing duplicate relationships...")
				
				go func() {
					defer fyne.Do(func() {
						duplicateRelBtn.Enable()
					})
					
					report, err := s.RemoveDuplicateRelationships()
					if err != nil {
						fyne.Do(func() {
							resultText.SetText(fmt.Sprintf("Error: %v", err))
						})
						return
					}
					
					fyne.Do(func() {
						displayMaintenanceReport(resultText, report)
					})
				}()
			}, w)
	})
	duplicateRelBtn.Importance = widget.WarningImportance
	
	duplicateRelDesc := widget.NewLabel("Removes duplicate relationship records.")
	duplicateRelDesc.Wrapping = fyne.TextWrapWord
	duplicateRelDesc.TextStyle.Italic = true
	
	// Layout
	operations := container.NewVBox(
		widget.NewLabel("Safe Operations:"),
		vacuumBtn,
		vacuumDesc,
		widget.NewSeparator(),
		integrityBtn,
		integrityDesc,
		widget.NewSeparator(),
		widget.NewLabel("Cleanup Operations (Use with caution):"),
		orphanedMediaBtn,
		orphanedMediaDesc,
		widget.NewSeparator(),
		orphanedRelBtn,
		orphanedRelDesc,
		widget.NewSeparator(),
		duplicateRelBtn,
		duplicateRelDesc,
	)
	
	// Make operations scrollable
	operationsScroll := container.NewVScroll(operations)
	operationsScroll.SetMinSize(fyne.NewSize(0, 300))
	
	// Results area - smaller but still readable
	resultScroll := container.NewScroll(resultText)
	resultScroll.SetMinSize(fyne.NewSize(0, 150))
	
	// Use a split container so both sections are accessible
	split := container.NewVSplit(
		operationsScroll,
		resultScroll,
	)
	split.SetOffset(0.70) // Give 70% to operations, 30% to results - more visible buttons
	
	return split
}

// displayMaintenanceReport formats and displays a maintenance report
func displayMaintenanceReport(label *widget.Label, report *store.MaintenanceReport) {
	var sb strings.Builder
	
	if report.Success {
		sb.WriteString("✅ ")
	} else {
		sb.WriteString("❌ ")
	}
	sb.WriteString(report.Operation)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("━", 50))
	sb.WriteString("\n\n")
	
	sb.WriteString(report.Message)
	sb.WriteString("\n\n")
	
	if report.ItemsRemoved > 0 {
		sb.WriteString(fmt.Sprintf("Items removed: %d\n", report.ItemsRemoved))
	}
	
	if report.SpaceReclaimed > 0 {
		sb.WriteString(fmt.Sprintf("Space reclaimed: %.2f MB\n", float64(report.SpaceReclaimed)/(1024*1024)))
	}
	
	if report.Duration > 0 {
		sb.WriteString(fmt.Sprintf("Duration: %v\n", report.Duration))
	}
	
	if len(report.Details) > 0 {
		sb.WriteString("\nDetails:\n")
		for _, detail := range report.Details {
			sb.WriteString(fmt.Sprintf("  • %s\n", detail))
		}
	}
	
	label.SetText(sb.String())
}

// createAnalysisTab creates the database analysis tab
func createAnalysisTab(w fyne.Window, s *store.Store) fyne.CanvasObject {
	resultText := widget.NewLabel("Select an analysis operation to view results.")
	resultText.Wrapping = fyne.TextWrapWord
	resultText.TextStyle.Monospace = true // Use monospace for better formatted output
	
	// Unused sources button
	var unusedSourcesBtn *widget.Button
	unusedSourcesBtn = widget.NewButton("📚 Find Unused Sources", func() {
		unusedSourcesBtn.Disable()
		resultText.SetText("Finding unused sources...")
		
		go func() {
			defer fyne.Do(func() {
				unusedSourcesBtn.Enable()
			})
			
			sources, err := s.GetUnusedSources()
			if err != nil {
				fyne.Do(func() {
					resultText.SetText(fmt.Sprintf("Error: %v", err))
				})
				return
			}
			
			var sb strings.Builder
			sb.WriteString("📚 UNUSED SOURCES\n")
			sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
			
			if len(sources) == 0 {
				sb.WriteString("✅ All sources are being used!\n")
			} else {
				sb.WriteString(fmt.Sprintf("Found %d unused source(s):\n\n", len(sources)))
				for i, source := range sources {
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, source.Title))
					if source.Author != "" {
						sb.WriteString(fmt.Sprintf("   Author: %s\n", source.Author))
					}
					if source.SourceType != "" {
						sb.WriteString(fmt.Sprintf("   Type: %s\n", source.SourceType))
					}
					sb.WriteString("\n")
				}
				sb.WriteString("💡 These sources have no citations. Consider adding citations or removing them.\n")
			}
			
			fyne.Do(func() {
				resultText.SetText(sb.String())
			})
		}()
	})
	
	unusedSourcesDesc := widget.NewLabel("Lists sources that have no citations linked to any person.")
	unusedSourcesDesc.Wrapping = fyne.TextWrapWord
	unusedSourcesDesc.TextStyle.Italic = true
	
	// Run analyze button
	var analyzeBtn *widget.Button
	analyzeBtn = widget.NewButton("📊 Optimize Query Performance", func() {
		analyzeBtn.Disable()
		resultText.SetText("Running ANALYZE...")
		
		go func() {
			defer fyne.Do(func() {
				analyzeBtn.Enable()
			})
			
			err := s.AnalyzeDatabase()
			if err != nil {
				fyne.Do(func() {
					resultText.SetText(fmt.Sprintf("Error: %v", err))
				})
				return
			}
			
			fyne.Do(func() {
				resultText.SetText("✅ Database query planner statistics updated successfully.\n\n" +
					"This helps SQLite optimize queries for better performance.")
			})
		}()
	})
	
	analyzeDesc := widget.NewLabel("Updates query planner statistics for better performance.")
	analyzeDesc.Wrapping = fyne.TextWrapWord
	analyzeDesc.TextStyle.Italic = true
	
	// Layout
	operations := container.NewVBox(
		unusedSourcesBtn,
		unusedSourcesDesc,
		widget.NewSeparator(),
		analyzeBtn,
		analyzeDesc,
	)
	
	return container.NewBorder(
		operations,
		nil, nil, nil,
		container.NewScroll(resultText),
	)
}
