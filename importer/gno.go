package importer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"genealogy/store"
)

// GenoPro XML structures
type GenoProDocument struct {
	XMLName     xml.Name            `xml:"GenoPro"`
	Individuals GenoProIndividuals  `xml:"Individuals"`
	Families    GenoProFamilies     `xml:"Families"`
	Marriages   GenoProMarriages    `xml:"Marriages"`
	Links       GenoPedgreeLinks    `xml:"PedigreeLinks"`
	Places      GenoProPlaces       `xml:"Places"`
}

type GenoProIndividuals struct {
	Individuals []GenoProIndividual `xml:"Individual"`
}

type GenoProIndividual struct {
	ID      string         `xml:"ID,attr"`
	Name    GenoProName    `xml:"Name"`
	Gender  string         `xml:"Gender"`
	Birth   *GenoProEvent  `xml:"Birth"`
	Death   *GenoProEvent  `xml:"Death"`
	IsDead  string         `xml:"IsDead"`
	Comment string         `xml:"Comment"`
}

type GenoProName struct {
	Display string `xml:"Display"`
	First   string `xml:"First"`
	Last    string `xml:"Last"`
}

type GenoProEvent struct {
	Date  string `xml:"Date"`
	Place string `xml:"Place"`
}

type GenoProFamilies struct {
	Families []GenoProFamily `xml:"Family"`
}

type GenoProFamily struct {
	ID       string `xml:"ID,attr"`
	Relation string `xml:"Relation"`
	Unions   string `xml:"Unions"`
}

type GenoProMarriages struct {
	Marriages []GenoProMarriage `xml:"Marriage"`
}

type GenoProMarriage struct {
	ID    string `xml:"ID,attr"`
	Type  string `xml:"Type"`
	Date  string `xml:"Date"`
	Place string `xml:"Place"`
}

type GenoPedgreeLinks struct {
	Links []GenoPedigreeLink `xml:"PedigreeLink"`
}

type GenoPedigreeLink struct {
	LinkType   string `xml:"PedigreeLink,attr"`
	Family     string `xml:"Family,attr"`
	Individual string `xml:"Individual,attr"`
}

type GenoProPlaces struct {
	Places []GenoProPlace `xml:"Place"`
}

type GenoProPlace struct {
	ID   string `xml:"ID,attr"`
	Name string `xml:"Name"`
}

// ImportGenoPro imports a GenoPro .gno file (which is a ZIP containing XML)
func ImportGenoPro(path string, s *store.Store) error {
	// Open the .gno file as a ZIP archive
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("failed to open .gno file: %w", err)
	}
	defer r.Close()

	// Find and read Data.xml from the archive
	var dataXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "Data.xml" {
			dataXML, err = f.Open()
			if err != nil {
				return fmt.Errorf("failed to open Data.xml: %w", err)
			}
			defer dataXML.Close()
			break
		}
	}

	if dataXML == nil {
		return fmt.Errorf("Data.xml not found in .gno archive")
	}

	// Parse the XML
	var doc GenoProDocument
	decoder := xml.NewDecoder(dataXML)
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	// Build place lookup map
	placeLookup := make(map[string]string)
	for _, place := range doc.Places.Places {
		placeLookup[place.ID] = place.Name
	}

	// Build marriage lookup map
	marriageLookup := make(map[string]*GenoProMarriage)
	for i := range doc.Marriages.Marriages {
		m := &doc.Marriages.Marriages[i]
		marriageLookup[m.ID] = m
	}

	// Track imported individuals and families
	indMap := make(map[string]int64)        // GenoPro ID -> DB ID
	famParents := make(map[string][]string) // Family ID -> []Parent IDs
	famChildren := make(map[string][]string) // Family ID -> []Child IDs
	
	// GenoPro uses different IDs for the same person across different GenoMaps
	// Deduplicate by name+birthdate instead
	type personKey struct {
		name      string
		birthDate string
	}
	seenPersons := make(map[personKey]int64) // person key -> DB ID

	// Import individuals (GenoPro can have duplicates across multiple GenoMaps)
	fmt.Printf("GenoPro: Found %d individual entries to process\n", len(doc.Individuals.Individuals))
	uniqueCount := 0
	for _, ind := range doc.Individuals.Individuals {
		// Build person data
		givenName := strings.TrimSpace(ind.Name.First)
		surname := strings.TrimSpace(ind.Name.Last)
		gender := strings.ToUpper(strings.TrimSpace(ind.Gender))
		
		var birthDate, birthPlace, deathDate, deathPlace string
		
		// Handle birth
		if ind.Birth != nil {
			birthDate = strings.TrimSpace(ind.Birth.Date)
			if ind.Birth.Place != "" {
				if placeName, ok := placeLookup[ind.Birth.Place]; ok {
					birthPlace = placeName
				} else {
					birthPlace = ind.Birth.Place
				}
			}
		}

		// Handle death
		if ind.Death != nil {
			deathDate = strings.TrimSpace(ind.Death.Date)
			if ind.Death.Place != "" {
				if placeName, ok := placeLookup[ind.Death.Place]; ok {
					deathPlace = placeName
				} else {
					deathPlace = ind.Death.Place
				}
			}
		}

		// Create deduplication key
		key := personKey{
			name:      strings.ToLower(givenName + " " + surname),
			birthDate: birthDate,
		}

		// Check if we've already imported this person
		if existingID, exists := seenPersons[key]; exists {
			// This is a duplicate - map this GenoPro ID to the existing DB ID
			indMap[ind.ID] = existingID
			continue
		}

		// New unique person - create in database
		person := &store.Person{
			GivenName:   givenName,
			Surname:     surname,
			Gender:      gender,
			BirthDate:   birthDate,
			BirthPlace:  birthPlace,
			DeathDate:   deathDate,
			DeathPlace:  deathPlace,
			IsLiving:    ind.IsDead != "Y",
		}

		// Comments/notes
		if ind.Comment != "" {
			person.Notes = strings.TrimSpace(ind.Comment)
		}

		// Create person in database
		if err := s.CreatePerson(person); err != nil {
			return fmt.Errorf("failed to create person %s: %w", ind.Name.Display, err)
		}

		// Track this person
		indMap[ind.ID] = person.ID
		seenPersons[key] = person.ID
		uniqueCount++
	}

	// Parse PedigreeLinks to build family structure
	fmt.Printf("GenoPro: Found %d PedigreeLinks\n", len(doc.Links.Links))
	for _, link := range doc.Links.Links {
		if link.LinkType == "Parent" {
			// This individual is a parent in this family
			famParents[link.Family] = append(famParents[link.Family], link.Individual)
		} else if link.LinkType == "Biological" {
			// This individual is a child in this family
			famChildren[link.Family] = append(famChildren[link.Family], link.Individual)
		}
	}
	fmt.Printf("GenoPro: Built %d families with parents, %d families with children\n", 
		len(famParents), len(famChildren))

	// Track created relationships to avoid duplicates
	type relKey struct {
		subjectID int64
		objectID  int64
		relType   string
	}
	createdRels := make(map[relKey]bool)
	skippedRels := 0

	// Create relationships
	for _, fam := range doc.Families.Families {
		parents := famParents[fam.ID]
		children := famChildren[fam.ID]

		// Handle spouse relationships
		if len(parents) == 2 {
			parent1ID := indMap[parents[0]]
			parent2ID := indMap[parents[1]]

			// Check if we've already created this spouse relationship
			relKey1 := relKey{parent1ID, parent2ID, "spouse"}
			relKey2 := relKey{parent2ID, parent1ID, "spouse"}
			
			if !createdRels[relKey1] {
				rel := &store.Relationship{
					SubjectID: parent1ID,
					ObjectID:  parent2ID,
					Type:      "spouse", // lowercase to match GEDCOM importer
				}

				// Handle marriage information
				if fam.Unions != "" {
					if marriage, ok := marriageLookup[fam.Unions]; ok {
						rel.MarriageDate = strings.TrimSpace(marriage.Date)
						if marriage.Place != "" {
							if placeName, ok := placeLookup[marriage.Place]; ok {
								rel.MarriagePlace = placeName
							} else {
								rel.MarriagePlace = marriage.Place
							}
						}
					}
				}

				// Handle divorce
				if fam.Relation == "Divorce" {
					rel.EndReason = "Divorced"
					// DivorceDate could be extracted if available in future versions
				}

				if err := s.CreateRelationship(rel); err != nil {
					// Might already exist due to unique constraint, that's OK
					if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
						// Log unexpected errors but don't fail the import
						fmt.Printf("Warning: failed to create spouse relationship: %v\n", err)
					} else {
						skippedRels++
					}
				} else {
					createdRels[relKey1] = true
				}
			}

			// Create inverse relationship
			if !createdRels[relKey2] {
				invRel := &store.Relationship{
					SubjectID:     parent2ID,
					ObjectID:      parent1ID,
					Type:          "spouse", // lowercase to match GEDCOM importer
				}
				
				// Copy marriage info for inverse relationship
				if fam.Unions != "" {
					if marriage, ok := marriageLookup[fam.Unions]; ok {
						invRel.MarriageDate = strings.TrimSpace(marriage.Date)
						if marriage.Place != "" {
							if placeName, ok := placeLookup[marriage.Place]; ok {
								invRel.MarriagePlace = placeName
							} else {
								invRel.MarriagePlace = marriage.Place
							}
						}
					}
				}
				if fam.Relation == "Divorce" {
					invRel.EndReason = "Divorced"
				}
				
				if err := s.CreateRelationship(invRel); err != nil {
					// Might already exist due to unique constraint, that's OK
					if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
						fmt.Printf("Warning: failed to create inverse spouse relationship: %v\n", err)
					} else {
						skippedRels++
					}
				} else {
					createdRels[relKey2] = true
				}
			}
		}

		// Handle parent-child relationships
		for _, parentGnoID := range parents {
			parentID, ok := indMap[parentGnoID]
			if !ok {
				continue
			}

			for _, childGnoID := range children {
				childID, ok := indMap[childGnoID]
				if !ok {
					continue
				}

				// Check if we've already created this parent-child relationship
				childRelKey := relKey{parentID, childID, "child"}
				parentRelKey := relKey{childID, parentID, "parent"}

				// Parent -> Child relationship (matches GEDCOM importer)
				if !createdRels[childRelKey] {
					rel := &store.Relationship{
						SubjectID: parentID,
						ObjectID:  childID,
						Type:      "child", // lowercase to match GEDCOM importer
					}

					if err := s.CreateRelationship(rel); err != nil {
						// Might already exist due to unique constraint, that's OK
						if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
							fmt.Printf("Warning: failed to create parent-child relationship: %v\n", err)
						} else {
							skippedRels++
						}
					} else {
						createdRels[childRelKey] = true
					}
				}

				// Child -> Parent relationship (inverse)
				if !createdRels[parentRelKey] {
					invRel := &store.Relationship{
						SubjectID: childID,
						ObjectID:  parentID,
						Type:      "parent", // lowercase to match GEDCOM importer
					}

					if err := s.CreateRelationship(invRel); err != nil {
						// Might already exist due to unique constraint, that's OK
						if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
							fmt.Printf("Warning: failed to create child-parent relationship: %v\n", err)
						} else {
							skippedRels++
						}
					} else {
						createdRels[parentRelKey] = true
					}
				}
			}
		}
	}

	fmt.Printf("GenoPro import complete: %d unique individuals imported (from %d total entries), %d families processed, %d unique relationships created (%d duplicates skipped)\n", 
		uniqueCount, len(doc.Individuals.Individuals), len(doc.Families.Families), len(createdRels), skippedRels)
	return nil
}

// ExportGenoPro would export to GenoPro format (not implemented yet)
func ExportGenoPro(path string, s *store.Store) error {
	return fmt.Errorf("GenoPro export not yet implemented")
}
