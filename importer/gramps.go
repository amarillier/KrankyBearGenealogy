package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"genealogy/store"
)

// GrampsPersonJSON represents the JSON structure stored in Gramps person.json_data
type GrampsPersonJSON struct {
	Handle           string                   `json:"handle"`
	GrampsID         string                   `json:"gramps_id"`
	Gender           int                      `json:"gender"`
	PrimaryName      GrampsName               `json:"primary_name"`
	EventRefList     []GrampsEventRef         `json:"event_ref_list"`
	FamilyList       []string                 `json:"family_list"`
	ParentFamilyList []string                 `json:"parent_family_list"`
	NoteList         []string                 `json:"note_list"`
	MediaList        []interface{}            `json:"media_list"`
	AttributeList    []GrampsAttribute        `json:"attribute_list"`
	BirthRefIndex    int                      `json:"birth_ref_index"`
	DeathRefIndex    int                      `json:"death_ref_index"`
}

type GrampsName struct {
	FirstName    string          `json:"first_name"`
	SurnameList  []GrampsSurname `json:"surname_list"`
}

type GrampsSurname struct {
	Surname string `json:"surname"`
}

type GrampsEventRef struct {
	Ref  string      `json:"ref"`
	Role GrampsRole  `json:"role"`
}

type GrampsRole struct {
	Value int `json:"value"`
}

type GrampsAttribute struct {
	Type  GrampsAttrType `json:"type"`
	Value string         `json:"value"`
}

type GrampsAttrType struct {
	String string `json:"string"`
}

// GrampsEventJSON represents the JSON structure stored in Gramps event.json_data
type GrampsEventJSON struct {
	Handle      string       `json:"handle"`
	GrampsID    string       `json:"gramps_id"`
	Type        GrampsType   `json:"type"`
	Date        GrampsDate   `json:"date"`
	Place       string       `json:"place"`
	Description string       `json:"description"`
}

type GrampsType struct {
	Value  int    `json:"value"`
	String string `json:"string"`
}

type GrampsDate struct {
	Dateval  []interface{} `json:"dateval"` // [year, month, day, boolean]
	Modifier int           `json:"modifier"`
	Quality  int           `json:"quality"`
	Text     string        `json:"text"`
}

// GrampsFamilyJSON represents the JSON structure stored in Gramps family.json_data
type GrampsFamilyJSON struct {
	Handle        string           `json:"handle"`
	GrampsID      string           `json:"gramps_id"`
	FatherHandle  string           `json:"father_handle"`
	MotherHandle  string           `json:"mother_handle"`
	ChildRefList  []GrampsChildRef `json:"child_ref_list"`
	EventRefList  []GrampsEventRef `json:"event_ref_list"`
}

type GrampsChildRef struct {
	Ref string `json:"ref"`
}

// ImportGramps imports data from a Gramps SQLite database
func ImportGramps(grampsDBPath string, targetStore *store.Store) error {
	// Open the Gramps database
	grampsDB, err := sql.Open("sqlite", grampsDBPath)
	if err != nil {
		return fmt.Errorf("failed to open Gramps database: %w", err)
	}
	defer grampsDB.Close()

	// Maps to track Gramps handles -> our IDs
	handleToPersonID := make(map[string]int64)
	handleToEventData := make(map[string]*GrampsEventJSON)
	handleToPlaceData := make(map[string]string)
	handleToNoteData := make(map[string]string)
	handleToMediaData := make(map[string]string)

	// Step 1: Load all events first (we'll need them for birth/death dates)
	if err := loadGrampsEvents(grampsDB, handleToEventData, handleToPlaceData); err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}

	// Step 2: Load all notes
	if err := loadGrampsNotes(grampsDB, handleToNoteData); err != nil {
		return fmt.Errorf("failed to load notes: %w", err)
	}

	// Step 3: Load all media references
	if err := loadGrampsMedia(grampsDB, handleToMediaData); err != nil {
		return fmt.Errorf("failed to load media: %w", err)
	}

	// Step 4: Import all people
	if err := importGrampsPeople(grampsDB, targetStore, handleToPersonID, handleToEventData, handleToPlaceData, handleToNoteData, handleToMediaData); err != nil {
		return fmt.Errorf("failed to import people: %w", err)
	}

	// Step 5: Import all families (relationships)
	if err := importGrampsFamilies(grampsDB, targetStore, handleToPersonID, handleToEventData, handleToPlaceData); err != nil {
		return fmt.Errorf("failed to import families: %w", err)
	}

	log.Printf("Gramps import complete: %d people imported", len(handleToPersonID))
	return nil
}

// loadGrampsEvents loads all events from Gramps database into memory
func loadGrampsEvents(db *sql.DB, handleToEventData map[string]*GrampsEventJSON, handleToPlaceData map[string]string) error {
	rows, err := db.Query("SELECT handle, json_data, place FROM event")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var handle, jsonData, placeHandle string
		if err := rows.Scan(&handle, &jsonData, &placeHandle); err != nil {
			log.Printf("Warning: failed to scan event: %v", err)
			continue
		}

		var event GrampsEventJSON
		if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
			log.Printf("Warning: failed to parse event JSON for handle %s: %v", handle, err)
			continue
		}

		handleToEventData[handle] = &event
		
		// Load place data if available
		if placeHandle != "" {
			place, err := loadGrampsPlace(db, placeHandle)
			if err == nil {
				handleToPlaceData[placeHandle] = place
			}
		}
	}

	log.Printf("Loaded %d events", len(handleToEventData))
	return nil
}

// loadGrampsPlace loads a place name from Gramps database
func loadGrampsPlace(db *sql.DB, placeHandle string) (string, error) {
	var jsonData string
	err := db.QueryRow("SELECT json_data FROM place WHERE handle = ?", placeHandle).Scan(&jsonData)
	if err != nil {
		return "", err
	}

	// Parse JSON to extract place name/title
	var placeData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &placeData); err != nil {
		return "", err
	}

	// Try to get title or name
	if title, ok := placeData["title"].(string); ok && title != "" {
		return title, nil
	}
	if name, ok := placeData["name"].(string); ok && name != "" {
		return name, nil
	}

	return "", fmt.Errorf("no place name found")
}

// loadGrampsNotes loads all notes from Gramps database into memory
func loadGrampsNotes(db *sql.DB, handleToNoteData map[string]string) error {
	rows, err := db.Query("SELECT handle, json_data FROM note")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var handle, jsonData string
		if err := rows.Scan(&handle, &jsonData); err != nil {
			log.Printf("Warning: failed to scan note: %v", err)
			continue
		}

		// Parse JSON to extract note text
		var noteData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &noteData); err != nil {
			log.Printf("Warning: failed to parse note JSON for handle %s: %v", handle, err)
			continue
		}

		// Extract text from note
		if text, ok := noteData["text"].(string); ok && text != "" {
			handleToNoteData[handle] = text
		}
	}

	log.Printf("Loaded %d notes", len(handleToNoteData))
	return nil
}

// loadGrampsMedia loads all media references from Gramps database into memory
func loadGrampsMedia(db *sql.DB, handleToMediaData map[string]string) error {
	rows, err := db.Query("SELECT handle, path, desc FROM media")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var handle, path, desc sql.NullString
		if err := rows.Scan(&handle, &path, &desc); err != nil {
			log.Printf("Warning: failed to scan media: %v", err)
			continue
		}

		// Store media path and description
		mediaInfo := ""
		if desc.Valid && desc.String != "" {
			mediaInfo = desc.String
		}
		if path.Valid && path.String != "" {
			if mediaInfo != "" {
				mediaInfo += " - "
			}
			mediaInfo += "File: " + path.String
		}

		if mediaInfo != "" {
			handleToMediaData[handle.String] = mediaInfo
		}
	}

	log.Printf("Loaded %d media references", len(handleToMediaData))
	return nil
}

// importGrampsPeople imports all person records from Gramps
func importGrampsPeople(db *sql.DB, targetStore *store.Store, handleToPersonID map[string]int64, 
	handleToEventData map[string]*GrampsEventJSON, handleToPlaceData map[string]string,
	handleToNoteData map[string]string, handleToMediaData map[string]string) error {
	
	rows, err := db.Query("SELECT handle, given_name, surname, gender, json_data FROM person")
	if err != nil {
		return err
	}
	defer rows.Close()

	imported := 0
	skipped := 0

	for rows.Next() {
		var handle, givenName, surname, jsonData string
		var gender sql.NullInt64
		if err := rows.Scan(&handle, &givenName, &surname, &gender, &jsonData); err != nil {
			log.Printf("Warning: failed to scan person: %v", err)
			continue
		}

		// Parse JSON for additional data
		var personData GrampsPersonJSON
		if err := json.Unmarshal([]byte(jsonData), &personData); err != nil {
			log.Printf("Warning: failed to parse person JSON for handle %s: %v", handle, err)
			continue
		}

		// Create person record
		person := &store.Person{
			GivenName: givenName,
			Surname:   surname,
			Gender:    mapGrampsGender(gender),
		}

		// Extract birth and death information from events
		if personData.BirthRefIndex >= 0 && personData.BirthRefIndex < len(personData.EventRefList) {
			birthEventHandle := personData.EventRefList[personData.BirthRefIndex].Ref
			if birthEvent, ok := handleToEventData[birthEventHandle]; ok {
				person.BirthDate = formatGrampsDate(birthEvent.Date)
				if birthEvent.Place != "" {
					if place, ok := handleToPlaceData[birthEvent.Place]; ok {
						person.BirthPlace = place
					}
				}
			}
		}

		if personData.DeathRefIndex >= 0 && personData.DeathRefIndex < len(personData.EventRefList) {
			deathEventHandle := personData.EventRefList[personData.DeathRefIndex].Ref
			if deathEvent, ok := handleToEventData[deathEventHandle]; ok {
				person.DeathDate = formatGrampsDate(deathEvent.Date)
				if deathEvent.Place != "" {
					if place, ok := handleToPlaceData[deathEvent.Place]; ok {
						person.DeathPlace = place
					}
				}
				person.IsLiving = false
			}
		} else {
			// No death date, assume living
			person.IsLiving = true
		}

		// Extract UID if available
		for _, attr := range personData.AttributeList {
			if attr.Type.String == "_UID" {
				person.UID = attr.Value
				break
			}
		}

		// Extract and combine notes
		var notesBuilder strings.Builder
		for _, noteHandle := range personData.NoteList {
			if noteText, ok := handleToNoteData[noteHandle]; ok {
				if notesBuilder.Len() > 0 {
					notesBuilder.WriteString("\n\n")
				}
				notesBuilder.WriteString(noteText)
			}
		}

		// Extract and append media references to notes (since we don't have media storage yet)
		if len(personData.MediaList) > 0 {
			if notesBuilder.Len() > 0 {
				notesBuilder.WriteString("\n\n")
			}
			notesBuilder.WriteString("--- Media Files ---\n")
			for _, mediaRef := range personData.MediaList {
				// MediaList contains media reference objects with a "ref" field
				if mediaRefMap, ok := mediaRef.(map[string]interface{}); ok {
					if ref, ok := mediaRefMap["ref"].(string); ok {
						if mediaInfo, ok := handleToMediaData[ref]; ok {
							notesBuilder.WriteString(mediaInfo)
							notesBuilder.WriteString("\n")
						}
					}
				}
			}
		}

		person.Notes = notesBuilder.String()

		// Check for duplicates (name + birthdate)
		existing, err := targetStore.GetPeople()
		if err != nil {
			return err
		}

		isDuplicate := false
		for _, e := range existing {
			if e.GivenName == person.GivenName && e.Surname == person.Surname && 
				e.BirthDate == person.BirthDate && person.BirthDate != "" {
				// Duplicate found, use existing ID
				handleToPersonID[handle] = e.ID
				isDuplicate = true
				skipped++
				break
			}
		}

		if !isDuplicate {
			// Create new person
			if err := targetStore.CreatePerson(person); err != nil {
				log.Printf("Warning: failed to create person %s %s: %v", givenName, surname, err)
				continue
			}
			handleToPersonID[handle] = person.ID
			imported++
		}
	}

	log.Printf("Imported %d people, skipped %d duplicates", imported, skipped)
	return nil
}

// importGrampsFamilies imports family relationships from Gramps
func importGrampsFamilies(db *sql.DB, targetStore *store.Store, handleToPersonID map[string]int64,
	handleToEventData map[string]*GrampsEventJSON, handleToPlaceData map[string]string) error {
	
	rows, err := db.Query("SELECT handle, father_handle, mother_handle, json_data FROM family")
	if err != nil {
		return err
	}
	defer rows.Close()

	spouseCount := 0
	childCount := 0

	for rows.Next() {
		var handle, jsonData string
		var fatherHandle, motherHandle sql.NullString
		if err := rows.Scan(&handle, &fatherHandle, &motherHandle, &jsonData); err != nil {
			log.Printf("Warning: failed to scan family: %v", err)
			continue
		}

		// Parse JSON for children and events
		var familyData GrampsFamilyJSON
		if err := json.Unmarshal([]byte(jsonData), &familyData); err != nil {
			log.Printf("Warning: failed to parse family JSON for handle %s: %v", handle, err)
			continue
		}

		// Create spouse relationship
		if fatherHandle.Valid && motherHandle.Valid {
			fatherID, fatherOK := handleToPersonID[fatherHandle.String]
			motherID, motherOK := handleToPersonID[motherHandle.String]

			if fatherOK && motherOK {
				// Find marriage event if exists
				var marriageDate, marriagePlace string
				for _, eventRef := range familyData.EventRefList {
					if event, ok := handleToEventData[eventRef.Ref]; ok {
						if event.Type.String == "Marriage" || strings.Contains(event.Description, "Marriage") {
							marriageDate = formatGrampsDate(event.Date)
							if event.Place != "" {
								if place, ok := handleToPlaceData[event.Place]; ok {
									marriagePlace = place
								}
							}
							break
						}
					}
				}

				// Create spouse relationship (bidirectional)
				rel1 := &store.Relationship{
					SubjectID:     fatherID,
					ObjectID:      motherID,
					Type:          "spouse",
					MarriageDate:  marriageDate,
					MarriagePlace: marriagePlace,
				}
				if err := targetStore.CreateRelationship(rel1); err != nil {
					// Might be duplicate, that's OK
					log.Printf("Note: Relationship already exists (father->mother)")
				} else {
					spouseCount++
				}

				rel2 := &store.Relationship{
					SubjectID:     motherID,
					ObjectID:      fatherID,
					Type:          "spouse",
					MarriageDate:  marriageDate,
					MarriagePlace: marriagePlace,
				}
				if err := targetStore.CreateRelationship(rel2); err != nil {
					log.Printf("Note: Relationship already exists (mother->father)")
				}
			}
		}

		// Create parent-child relationships
		for _, childRef := range familyData.ChildRefList {
			childID, childOK := handleToPersonID[childRef.Ref]
			if !childOK {
				continue
			}

			// Link child to father
			if fatherHandle.Valid {
				if fatherID, fatherOK := handleToPersonID[fatherHandle.String]; fatherOK {
					rel := &store.Relationship{
						SubjectID: fatherID,
						ObjectID:  childID,
						Type:      "child",
					}
					if err := targetStore.CreateRelationship(rel); err == nil {
						childCount++
					}

					// Reverse relationship (child -> parent)
					relReverse := &store.Relationship{
						SubjectID: childID,
						ObjectID:  fatherID,
						Type:      "parent",
					}
					targetStore.CreateRelationship(relReverse)
				}
			}

			// Link child to mother
			if motherHandle.Valid {
				if motherID, motherOK := handleToPersonID[motherHandle.String]; motherOK {
					rel := &store.Relationship{
						SubjectID: motherID,
						ObjectID:  childID,
						Type:      "child",
					}
					if err := targetStore.CreateRelationship(rel); err == nil {
						childCount++
					}

					// Reverse relationship (child -> parent)
					relReverse := &store.Relationship{
						SubjectID: childID,
						ObjectID:  motherID,
						Type:      "parent",
					}
					targetStore.CreateRelationship(relReverse)
				}
			}
		}
	}

	log.Printf("Created %d spouse relationships, %d parent-child relationships", spouseCount, childCount)
	return nil
}

// mapGrampsGender converts Gramps gender (0=Female, 1=Male, 2=Unknown) to our format
func mapGrampsGender(gender sql.NullInt64) string {
	if !gender.Valid {
		return "Unknown"
	}
	switch gender.Int64 {
	case 0:
		return "Female"
	case 1:
		return "Male"
	default:
		return "Unknown"
	}
}

// formatGrampsDate converts Gramps date format to YYYY-MM-DD string
func formatGrampsDate(date GrampsDate) string {
	if len(date.Dateval) < 3 {
		return ""
	}

	// Gramps stores dates as [year, month, day, boolean]
	year, okYear := date.Dateval[0].(float64)
	month, okMonth := date.Dateval[1].(float64)
	day, okDay := date.Dateval[2].(float64)

	if !okYear || year == 0 {
		return ""
	}

	// Handle modifiers (about, before, after)
	prefix := ""
	switch date.Modifier {
	case 1:
		prefix = "Bef " // Before
	case 2:
		prefix = "Aft " // After
	case 3:
		prefix = "Abt " // About
	}

	// Format date
	if okMonth && month > 0 {
		if okDay && day > 0 {
			return fmt.Sprintf("%s%04d-%02d-%02d", prefix, int(year), int(month), int(day))
		}
		return fmt.Sprintf("%s%04d-%02d", prefix, int(year), int(month))
	}
	return fmt.Sprintf("%s%04d", prefix, int(year))
}
