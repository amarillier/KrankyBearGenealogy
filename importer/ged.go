package importer

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"genealogy/store"
)

// Extended GEDCOM importer with full date support and marriage tracking.

var indiRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+INDI`)
var famRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+FAM`)
var tagRe = regexp.MustCompile(`^(\d+)\s+([A-Z0-9_]+)\s*(.*)$`)

type gedIndi struct {
	id         string
	name       string
	sex        string
	uid        string
	notes      string
	birthDate  string
	birthPlace string
	deathDate  string
	deathPlace string
	isLiving   bool
}

type gedFam struct {
	id            string
	husb          string
	wife          string
	children      []string
	marriageDate  string
	marriagePlace string
	divorceDate   string
}

// parseGedcomDate converts GEDCOM date format to a more readable format.
// GEDCOM uses: "DD MMM YYYY" (e.g., "3 DEC 1931")
// We'll keep it as is for maximum compatibility.
func parseGedcomDate(gedDate string) string {
	return strings.TrimSpace(gedDate)
}

// Import reads a GEDCOM file and populates the store with persons and relationships.
func Import(path string, s *store.Store) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var current string
	var currentType string // "INDI" or "FAM"
	indis := map[string]*gedIndi{}
	fams := map[string]*gedFam{}

	// Track context for nested tags
	var inBirth, inDeath, inMarriage, inDivorce bool
	var currentIndi *gedIndi
	var currentFam *gedFam

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for new INDI record
		if m := indiRe.FindStringSubmatch(line); len(m) == 2 {
			current = m[1]
			currentType = "INDI"
			indis[current] = &gedIndi{id: current}
			currentIndi = indis[current]
			currentFam = nil
			inBirth = false
			inDeath = false
			inMarriage = false
			continue
		}

		// Check for new FAM record
		if m := famRe.FindStringSubmatch(line); len(m) == 2 {
			current = m[1]
			currentType = "FAM"
			fams[current] = &gedFam{id: current}
			currentFam = fams[current]
			currentIndi = nil
			inBirth = false
			inDeath = false
			inMarriage = false
			continue
		}

		// Parse tag line
		m := tagRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		level := m[1]
		tag := m[2]
		rest := strings.TrimSpace(m[3])

		// Handle INDI context
		if currentType == "INDI" && currentIndi != nil {
			switch tag {
			case "NAME":
				currentIndi.name = strings.ReplaceAll(rest, "/", "")
			case "SEX":
				currentIndi.sex = rest
			case "UID", "_UID", "REFN":
				currentIndi.uid = rest
			case "NOTE":
				if currentIndi.notes != "" {
					currentIndi.notes += "\n"
				}
				currentIndi.notes += rest
			case "BIRT":
				inBirth = true
				inDeath = false
			case "DEAT":
				inDeath = true
				inBirth = false
				// If DEAT tag exists but no date follows, person is deceased
				currentIndi.isLiving = false
			case "DATE":
				if level == "2" {
					if inBirth {
						currentIndi.birthDate = parseGedcomDate(rest)
					} else if inDeath {
						currentIndi.deathDate = parseGedcomDate(rest)
						currentIndi.isLiving = false
					}
				}
			case "PLAC":
				if level == "2" {
					if inBirth {
						currentIndi.birthPlace = rest
					} else if inDeath {
						currentIndi.deathPlace = rest
					}
				}
			}
		}

		// Handle FAM context
		if currentType == "FAM" && currentFam != nil {
			switch tag {
			case "HUSB":
				currentFam.husb = rest
			case "WIFE":
				currentFam.wife = rest
			case "CHIL":
				currentFam.children = append(currentFam.children, rest)
			case "MARR":
				inMarriage = true
				inDivorce = false
			case "DIV":
				inDivorce = true
				inMarriage = false
			case "DATE":
				if level == "2" {
					if inMarriage {
						currentFam.marriageDate = parseGedcomDate(rest)
					} else if inDivorce {
						currentFam.divorceDate = parseGedcomDate(rest)
					}
				}
			case "PLAC":
				if level == "2" && inMarriage {
					currentFam.marriagePlace = rest
				}
			}
		}

		// Reset context flags when we hit a level 1 tag
		if level == "1" && tag != "BIRT" && tag != "DEAT" && tag != "MARR" && tag != "DIV" {
			inBirth = false
			inDeath = false
			inMarriage = false
			inDivorce = false
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Create persons and map GEDCOM id -> DB person id
	idMap := map[string]int64{}
	for gid, gi := range indis {
		// Split name into given and surname
		given := gi.name
		surname := ""
		parts := strings.Fields(gi.name)
		if len(parts) > 1 {
			surname = parts[len(parts)-1]
			given = strings.Join(parts[:len(parts)-1], " ")
		}

		p := &store.Person{
			GivenName:  given,
			Surname:    surname,
			Gender:     gi.sex,
			BirthDate:  gi.birthDate,
			BirthPlace: gi.birthPlace,
			DeathDate:  gi.deathDate,
			DeathPlace: gi.deathPlace,
			IsLiving:   gi.isLiving,
			UID:        gi.uid,
			Notes:      gi.notes,
		}

		if err := s.CreatePerson(p); err != nil {
			return fmt.Errorf("create person %s: %w", gid, err)
		}
		idMap[gid] = p.ID
	}

	// Create relationships from families
	for _, fam := range fams {
		// Create spouse relationship with marriage info
		if fam.husb != "" && fam.wife != "" {
			hid := strings.Trim(fam.husb, "@")
			wid := strings.Trim(fam.wife, "@")
			if h, ok1 := idMap[hid]; ok1 {
				if w, ok2 := idMap[wid]; ok2 {
					rel := &store.Relationship{
						SubjectID:     h,
						ObjectID:      w,
						Type:          "spouse",
						MarriageDate:  fam.marriageDate,
						MarriagePlace: fam.marriagePlace,
						DivorceDate:   fam.divorceDate,
					}
					if fam.divorceDate != "" {
						rel.EndReason = "divorce"
					}
					_ = s.CreateRelationship(rel)
				}
			}
		}

		// Create parent-child relationships
		for _, child := range fam.children {
			cid := strings.Trim(child, "@")
			childID, okc := idMap[cid]
			if !okc {
				continue
			}

			// Link to father
			if fam.husb != "" {
				pid := strings.Trim(fam.husb, "@")
				if parentID, okp := idMap[pid]; okp {
					_ = s.CreateRelationship(&store.Relationship{
						SubjectID: parentID,
						ObjectID:  childID,
						Type:      "child",
					})
				}
			}

			// Link to mother
			if fam.wife != "" {
				pid := strings.Trim(fam.wife, "@")
				if parentID, okp := idMap[pid]; okp {
					_ = s.CreateRelationship(&store.Relationship{
						SubjectID: parentID,
						ObjectID:  childID,
						Type:      "child",
					})
				}
			}
		}
	}

	return nil
}

// Export writes persons and relationships to a GEDCOM file.
func Export(path string, s *store.Store) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	people, err := s.GetPeople()
	if err != nil {
		return err
	}

	// Write GEDCOM header
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "0 HEAD\n")
	fmt.Fprintf(w, "1 SOUR KrankyBear Genealogy\n")
	fmt.Fprintf(w, "1 GEDC\n")
	fmt.Fprintf(w, "2 VERS 5.5\n")
	fmt.Fprintf(w, "2 FORM LINEAGE-LINKED\n")
	fmt.Fprintf(w, "1 CHAR UTF-8\n")

	// Map person id -> GEDCOM id @I1@ style
	gid := map[int64]string{}
	for i, p := range people {
		gid[p.ID] = fmt.Sprintf("I%d", i+1)
	}

	// Write individuals
	for _, p := range people {
		fmt.Fprintf(w, "0 @%s@ INDI\n", gid[p.ID])

		// Name
		name := strings.TrimSpace(p.GivenName)
		if p.Surname != "" {
			name += " /" + p.Surname + "/"
		}
		fmt.Fprintf(w, "1 NAME %s\n", name)

		// Sex
		if p.Gender != "" {
			fmt.Fprintf(w, "1 SEX %s\n", p.Gender)
		}

		// Birth
		if p.BirthDate != "" || p.BirthPlace != "" {
			fmt.Fprintf(w, "1 BIRT\n")
			if p.BirthDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", p.BirthDate)
			}
			if p.BirthPlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", p.BirthPlace)
			}
		}

		// Death
		if p.DeathDate != "" || p.DeathPlace != "" || !p.IsLiving {
			fmt.Fprintf(w, "1 DEAT\n")
			if p.DeathDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", p.DeathDate)
			}
			if p.DeathPlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", p.DeathPlace)
			}
		}

		// UID
		if p.UID != "" {
			fmt.Fprintf(w, "1 UID %s\n", p.UID)
		}

		// Notes
		if p.Notes != "" {
			// Split multi-line notes
			lines := strings.Split(p.Notes, "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(w, "1 NOTE %s\n", line)
				}
			}
		}
	}

	// Build families from spouse relationships
	rels, err := s.GetRelationships()
	if err != nil {
		return err
	}

	// Track which spouse pairs we've already created families for
	processedPairs := map[string]bool{}
	famIndex := 1

	for _, r := range rels {
		if r.Type != "spouse" {
			continue
		}

		// Create a unique key for this pair (order-independent)
		pairKey := fmt.Sprintf("%d-%d", min(r.SubjectID, r.ObjectID), max(r.SubjectID, r.ObjectID))
		if processedPairs[pairKey] {
			continue
		}
		processedPairs[pairKey] = true

		fid := fmt.Sprintf("F%d", famIndex)
		famIndex++

		fmt.Fprintf(w, "0 @%s@ FAM\n", fid)

		// Determine husband/wife by gender if possible
		person1, _ := s.GetPersonByID(r.SubjectID)
		person2, _ := s.GetPersonByID(r.ObjectID)

		if person1 != nil && person2 != nil {
			if person1.Gender == "M" {
				fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.SubjectID])
				fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.ObjectID])
			} else {
				fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.ObjectID])
				fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.SubjectID])
			}
		} else {
			// Fallback if we can't determine gender
			fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.SubjectID])
			fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.ObjectID])
		}

		// Marriage date/place
		if r.MarriageDate != "" || r.MarriagePlace != "" {
			fmt.Fprintf(w, "1 MARR\n")
			if r.MarriageDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", r.MarriageDate)
			}
			if r.MarriagePlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", r.MarriagePlace)
			}
		}

		// Divorce date
		if r.DivorceDate != "" {
			fmt.Fprintf(w, "1 DIV\n")
			fmt.Fprintf(w, "2 DATE %s\n", r.DivorceDate)
		}

		// Find children of this couple
		for _, rr := range rels {
			if rr.Type == "child" {
				// Check if this child belongs to either parent
				if rr.SubjectID == r.SubjectID || rr.SubjectID == r.ObjectID {
					childGID, ok := gid[rr.ObjectID]
					if ok {
						fmt.Fprintf(w, "1 CHIL @%s@\n", childGID)
					}
				}
			}
		}
	}

	// Trailer
	fmt.Fprintf(w, "0 TRLR\n")

	return w.Flush()
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// ExportBranch exports only a specific branch of the family tree starting from rootPersonID
// and including all descendants (children, grandchildren, etc.) and their spouses
func ExportBranch(path string, s *store.Store, rootPersonID int64) error {
	// Get all descendants recursively
	descendantIDs := make(map[int64]bool)
	descendantIDs[rootPersonID] = true // Include the root person
	
	// Recursively collect all descendants
	collectDescendants(s, rootPersonID, descendantIDs)
	
	// Also include spouses of all descendants
	spouseIDs := make(map[int64]bool)
	for personID := range descendantIDs {
		spouses, _ := s.GetSpouses(personID)
		for _, sp := range spouses {
			spouseIDs[sp.Person.ID] = true
		}
	}
	
	// Merge spouses into descendants
	for spouseID := range spouseIDs {
		descendantIDs[spouseID] = true
	}
	
	// Get all people and filter to only include descendants
	allPeople, err := s.GetPeople()
	if err != nil {
		return err
	}
	
	var people []store.Person
	for _, p := range allPeople {
		if descendantIDs[p.ID] {
			people = append(people, p)
		}
	}
	
	// Now export using the filtered people list
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Write GEDCOM header
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "0 HEAD\n")
	fmt.Fprintf(w, "1 SOUR KrankyBear Genealogy\n")
	fmt.Fprintf(w, "1 GEDC\n")
	fmt.Fprintf(w, "2 VERS 5.5\n")
	fmt.Fprintf(w, "2 FORM LINEAGE-LINKED\n")
	fmt.Fprintf(w, "1 CHAR UTF-8\n")
	
	// Map person id -> GEDCOM id @I1@ style
	gid := map[int64]string{}
	for i, p := range people {
		gid[p.ID] = fmt.Sprintf("I%d", i+1)
	}
	
	// Write individuals
	for _, p := range people {
		fmt.Fprintf(w, "0 @%s@ INDI\n", gid[p.ID])
		
		// Name
		name := strings.TrimSpace(p.GivenName)
		if p.Surname != "" {
			name += " /" + p.Surname + "/"
		}
		fmt.Fprintf(w, "1 NAME %s\n", name)
		
		// Sex
		if p.Gender != "" {
			fmt.Fprintf(w, "1 SEX %s\n", p.Gender)
		}
		
		// Birth
		if p.BirthDate != "" || p.BirthPlace != "" {
			fmt.Fprintf(w, "1 BIRT\n")
			if p.BirthDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", p.BirthDate)
			}
			if p.BirthPlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", p.BirthPlace)
			}
		}
		
		// Death
		if p.DeathDate != "" || p.DeathPlace != "" || !p.IsLiving {
			fmt.Fprintf(w, "1 DEAT\n")
			if p.DeathDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", p.DeathDate)
			}
			if p.DeathPlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", p.DeathPlace)
			}
		}
		
		// UID
		if p.UID != "" {
			fmt.Fprintf(w, "1 UID %s\n", p.UID)
		}
		
		// Notes
		if p.Notes != "" {
			lines := strings.Split(p.Notes, "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(w, "1 NOTE %s\n", line)
				}
			}
		}
	}
	
	// Build families from spouse relationships (only for people in our filtered list)
	rels, err := s.GetRelationships()
	if err != nil {
		return err
	}
	
	processedPairs := map[string]bool{}
	famIndex := 1
	
	for _, r := range rels {
		if r.Type != "spouse" {
			continue
		}
		
		// Only include this family if both partners are in our filtered list
		if !descendantIDs[r.SubjectID] || !descendantIDs[r.ObjectID] {
			continue
		}
		
		pairKey := fmt.Sprintf("%d-%d", min(r.SubjectID, r.ObjectID), max(r.SubjectID, r.ObjectID))
		if processedPairs[pairKey] {
			continue
		}
		processedPairs[pairKey] = true
		
		fid := fmt.Sprintf("F%d", famIndex)
		famIndex++
		
		fmt.Fprintf(w, "0 @%s@ FAM\n", fid)
		
		person1, _ := s.GetPersonByID(r.SubjectID)
		person2, _ := s.GetPersonByID(r.ObjectID)
		
		if person1 != nil && person2 != nil {
			if person1.Gender == "M" {
				fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.SubjectID])
				fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.ObjectID])
			} else {
				fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.ObjectID])
				fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.SubjectID])
			}
		} else {
			fmt.Fprintf(w, "1 HUSB @%s@\n", gid[r.SubjectID])
			fmt.Fprintf(w, "1 WIFE @%s@\n", gid[r.ObjectID])
		}
		
		// Marriage date/place
		if r.MarriageDate != "" || r.MarriagePlace != "" {
			fmt.Fprintf(w, "1 MARR\n")
			if r.MarriageDate != "" {
				fmt.Fprintf(w, "2 DATE %s\n", r.MarriageDate)
			}
			if r.MarriagePlace != "" {
				fmt.Fprintf(w, "2 PLAC %s\n", r.MarriagePlace)
			}
		}
		
		// Divorce date
		if r.DivorceDate != "" {
			fmt.Fprintf(w, "1 DIV\n")
			fmt.Fprintf(w, "2 DATE %s\n", r.DivorceDate)
		}
		
		// Find children of this couple (only if they're in our filtered list)
		for _, rr := range rels {
			if rr.Type == "child" {
				if rr.SubjectID == r.SubjectID || rr.SubjectID == r.ObjectID {
					if descendantIDs[rr.ObjectID] {
						childGID, ok := gid[rr.ObjectID]
						if ok {
							fmt.Fprintf(w, "1 CHIL @%s@\n", childGID)
						}
					}
				}
			}
		}
	}
	
	// Trailer
	fmt.Fprintf(w, "0 TRLR\n")
	
	return w.Flush()
}

// collectDescendants recursively collects all descendant IDs
func collectDescendants(s *store.Store, personID int64, descendants map[int64]bool) {
	children, err := s.GetRelatedPeople(personID, "child")
	if err != nil {
		return
	}
	
	for _, child := range children {
		if !descendants[child.ID] {
			descendants[child.ID] = true
			// Recursively get their descendants
			collectDescendants(s, child.ID, descendants)
		}
	}
}
