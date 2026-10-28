package importer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"genealogy/store"
)

// GEDCOM parsing is separate from writing to the database so a file can be
// read into memory and inspected (e.g. for merging) before anything is stored.

var indiRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+INDI`)
var famRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+FAM`)
var tagRe = regexp.MustCompile(`^(\d+)\s+([A-Z0-9_]+)\s*(.*)$`)

// ParsedPerson is an INDI record read from a GEDCOM file.
type ParsedPerson struct {
	GedID  string        // GEDCOM xref without the surrounding @ signs
	Person *store.Person // ID is zero until written to a store
}

// ParsedFamily is a FAM record read from a GEDCOM file. Husband, Wife and
// Children hold GEDCOM xrefs without the surrounding @ signs.
type ParsedFamily struct {
	GedID         string
	Husband       string
	Wife          string
	Children      []string
	MarriageDate  string
	MarriagePlace string
	DivorceDate   string
}

// ParsedGedcom is the in-memory content of a GEDCOM file, in file order.
type ParsedGedcom struct {
	People   []*ParsedPerson
	Families []*ParsedFamily
}

// parseGedcomDate converts GEDCOM date format to a more readable format.
// GEDCOM uses: "DD MMM YYYY" (e.g., "3 DEC 1931")
// We'll keep it as is for maximum compatibility.
func parseGedcomDate(gedDate string) string {
	return strings.TrimSpace(gedDate)
}

// splitGedcomName splits a GEDCOM NAME value into given name and surname.
// The standard form is "Given /Surname/ Suffix"; names without slashes fall
// back to treating the last word as the surname.
func splitGedcomName(raw string) (given, surname string) {
	raw = strings.TrimSpace(raw)
	if open := strings.Index(raw, "/"); open >= 0 {
		rest := raw[open+1:]
		closeIdx := strings.Index(rest, "/")
		if closeIdx < 0 {
			closeIdx = len(rest)
		}
		surname = strings.TrimSpace(rest[:closeIdx])
		after := ""
		if closeIdx < len(rest) {
			after = rest[closeIdx+1:]
		}
		given = strings.Join(strings.Fields(raw[:open]+" "+after), " ")
		return given, surname
	}
	parts := strings.Fields(raw)
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
	return strings.Join(parts, " "), ""
}

// continuationValue returns the value of a CONC/CONT line, preserving any
// leading space beyond the single delimiter after the tag.
func continuationValue(line string) string {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

// ParseGedcom reads a GEDCOM file into memory without touching any database.
func ParseGedcom(path string) (*ParsedGedcom, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseGedcomReader(f)
}

// ParseGedcomReader parses GEDCOM content from r.
func ParseGedcomReader(r io.Reader) (*ParsedGedcom, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	result := &ParsedGedcom{}
	var currentIndi *ParsedPerson
	var currentFam *ParsedFamily
	var nameSeen, lastWasNote bool

	// Track context for nested tags
	var inBirth, inDeath, inMarriage, inDivorce bool
	resetContext := func() {
		inBirth, inDeath, inMarriage, inDivorce = false, false, false, false
	}

	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			line = strings.TrimPrefix(line, "\xef\xbb\xbf")
			first = false
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		if m := indiRe.FindStringSubmatch(line); len(m) == 2 {
			currentIndi = &ParsedPerson{GedID: m[1], Person: &store.Person{}}
			result.People = append(result.People, currentIndi)
			currentFam = nil
			nameSeen, lastWasNote = false, false
			resetContext()
			continue
		}

		if m := famRe.FindStringSubmatch(line); len(m) == 2 {
			currentFam = &ParsedFamily{GedID: m[1]}
			result.Families = append(result.Families, currentFam)
			currentIndi = nil
			lastWasNote = false
			resetContext()
			continue
		}

		m := tagRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		level := m[1]
		tag := m[2]
		rest := strings.TrimSpace(m[3])

		// Any other level-0 record (HEAD, SOUR, TRLR...) ends the current one
		if level == "0" {
			currentIndi, currentFam = nil, nil
			lastWasNote = false
			resetContext()
			continue
		}

		if currentIndi != nil {
			p := currentIndi.Person
			switch tag {
			case "NAME":
				// Only the first NAME is the primary name; later ones are aliases
				if level == "1" && !nameSeen {
					nameSeen = true
					p.GivenName, p.Surname = splitGedcomName(m[3])
				}
			case "SEX":
				if level == "1" {
					p.Gender = rest
				}
			case "UID", "_UID", "REFN":
				if level == "1" {
					p.UID = rest
				}
			case "NOTE":
				if level == "1" {
					if p.Notes != "" {
						p.Notes += "\n"
					}
					p.Notes += rest
					lastWasNote = true
				}
			case "CONC", "CONT":
				if lastWasNote && level == "2" {
					if tag == "CONT" {
						p.Notes += "\n"
					}
					p.Notes += continuationValue(line)
				}
			case "BIRT":
				inBirth, inDeath = true, false
			case "DEAT":
				inDeath, inBirth = true, false
			case "DATE":
				if level == "2" {
					if inBirth {
						p.BirthDate = parseGedcomDate(rest)
					} else if inDeath {
						p.DeathDate = parseGedcomDate(rest)
					}
				}
			case "PLAC":
				if level == "2" {
					if inBirth {
						p.BirthPlace = rest
					} else if inDeath {
						p.DeathPlace = rest
					}
				}
			}
			// IsLiving stays false (the historical import default); a person
			// with no death information is not inferred to be living here.
		}

		if currentFam != nil {
			switch tag {
			case "HUSB":
				currentFam.Husband = strings.Trim(rest, "@")
			case "WIFE":
				currentFam.Wife = strings.Trim(rest, "@")
			case "CHIL":
				currentFam.Children = append(currentFam.Children, strings.Trim(rest, "@"))
			case "MARR":
				inMarriage, inDivorce = true, false
			case "DIV":
				inDivorce, inMarriage = true, false
			case "DATE":
				if level == "2" {
					if inMarriage {
						currentFam.MarriageDate = parseGedcomDate(rest)
					} else if inDivorce {
						currentFam.DivorceDate = parseGedcomDate(rest)
					}
				}
			case "PLAC":
				if level == "2" && inMarriage {
					currentFam.MarriagePlace = rest
				}
			}
		}

		// Reset context flags when we hit a level 1 tag
		if level == "1" {
			if tag != "NOTE" {
				lastWasNote = false
			}
			if tag != "BIRT" && tag != "DEAT" && tag != "MARR" && tag != "DIV" {
				resetContext()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// Import reads a GEDCOM file and populates the store with persons and relationships.
func Import(path string, s *store.Store) error {
	parsed, err := ParseGedcom(path)
	if err != nil {
		return err
	}
	_, err = writeParsed(parsed, s)
	return err
}

// writeParsed stores every person and family from parsed and returns the
// GEDCOM xref -> database person ID mapping.
func writeParsed(parsed *ParsedGedcom, s *store.Store) (map[string]int64, error) {
	idMap := map[string]int64{}
	for _, pp := range parsed.People {
		if err := s.CreatePerson(pp.Person); err != nil {
			return nil, fmt.Errorf("create person %s: %w", pp.GedID, err)
		}
		idMap[pp.GedID] = pp.Person.ID
	}

	for _, fam := range parsed.Families {
		if fam.Husband != "" && fam.Wife != "" {
			if h, ok := idMap[fam.Husband]; ok {
				if w, ok := idMap[fam.Wife]; ok {
					rel := &store.Relationship{
						SubjectID:     h,
						ObjectID:      w,
						Type:          "spouse",
						MarriageDate:  fam.MarriageDate,
						MarriagePlace: fam.MarriagePlace,
						DivorceDate:   fam.DivorceDate,
					}
					if fam.DivorceDate != "" {
						rel.EndReason = "divorce"
					}
					_ = s.CreateRelationship(rel)
				}
			}
		}

		for _, child := range fam.Children {
			childID, ok := idMap[child]
			if !ok {
				continue
			}
			for _, parent := range []string{fam.Husband, fam.Wife} {
				if parentID, ok := idMap[parent]; ok && parent != "" {
					_ = s.CreateRelationship(&store.Relationship{
						SubjectID: parentID,
						ObjectID:  childID,
						Type:      "child",
					})
				}
			}
		}
	}
	return idMap, nil
}
