package importer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"genealogy/internal/dedupe"
	"genealogy/store"
)

// GEDCOM parsing is separate from writing to the database so a file can be
// read into memory and inspected (e.g. for merging) before anything is stored.

var indiRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+INDI`)
var famRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+FAM`)
var srcRe = regexp.MustCompile(`^0\s+@([^@]+)@\s+SOUR`)
var tagRe = regexp.MustCompile(`^(\d+)\s+([A-Z0-9_]+)\s*(.*)$`)

// ParsedPerson is an INDI record read from a GEDCOM file.
type ParsedPerson struct {
	GedID  string        // GEDCOM xref without the surrounding @ signs
	Person *store.Person // ID is zero until written to a store
	// HasDeath is true if the record has a DEAT tag, even with no date ("1 DEAT Y").
	HasDeath bool
	// AltNames are extra NAME/NICK records; PersonID is unset until stored.
	AltNames  []store.AlternateName
	Citations []*ParsedCitation
}

// ParsedCitation is a SOUR reference on a person. Either SourceXref points at a
// top-level source record, or InlineTitle holds a source written in place.
type ParsedCitation struct {
	SourceXref    string
	InlineTitle   string
	Detail        string // PAGE
	Transcription string // DATA/TEXT
	Confidence    string // low, medium or high, from QUAY
	Context       string // "Birth" or "Death" when cited on that event
}

// livingCutoffYears is how old a person with no death information can be and
// still be assumed living. It matches the Living Status Report's threshold.
const livingCutoffYears = 100

// now is the clock used to infer living status; tests replace it.
var now = time.Now

// inferLiving reports whether a person should be imported as living: no death
// information of any kind, and a known birth year within the cutoff. With no
// usable birth date there is nothing to go on, so the person is not marked living.
func inferLiving(pp *ParsedPerson) bool {
	p := pp.Person
	if pp.HasDeath || strings.TrimSpace(p.DeathDate) != "" || strings.TrimSpace(p.DeathPlace) != "" {
		return false
	}
	year := dedupe.ParseDate(p.BirthDate).Year
	if year == 0 {
		return false
	}
	return now().Year()-year < livingCutoffYears
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
	// Sources are the top-level SOUR records keyed by xref.
	Sources map[string]*store.Source
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

// altNameType maps a GEDCOM name TYPE to one of the app's alternate name types.
func altNameType(gedType string) string {
	switch strings.ToLower(strings.TrimSpace(gedType)) {
	case "maiden", "birth":
		return "maiden"
	case "married":
		return "married"
	case "nickname":
		return "nickname"
	}
	return "other"
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

	result := &ParsedGedcom{Sources: map[string]*store.Source{}}
	var currentIndi *ParsedPerson
	var currentFam *ParsedFamily
	var currentSrc *store.Source
	var srcLast *string // source field a following CONC/CONT line extends
	var nameSeen, lastWasNote bool
	var curCit *ParsedCitation
	citLevel := 0
	curAlt := -1 // index in the current person's AltNames of the NAME being read

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
			currentFam, currentSrc = nil, nil
			nameSeen, lastWasNote, curCit, curAlt = false, false, nil, -1
			resetContext()
			continue
		}

		if m := famRe.FindStringSubmatch(line); len(m) == 2 {
			currentFam = &ParsedFamily{GedID: m[1]}
			result.Families = append(result.Families, currentFam)
			currentIndi, currentSrc = nil, nil
			lastWasNote, curCit = false, nil
			resetContext()
			continue
		}

		if m := srcRe.FindStringSubmatch(line); len(m) == 2 {
			currentSrc = &store.Source{SourceType: "other"}
			result.Sources[m[1]] = currentSrc
			currentIndi, currentFam, srcLast = nil, nil, nil
			lastWasNote, curCit = false, nil
			resetContext()
			continue
		}

		// Any other level-0 record (HEAD, TRLR, NOTE, REPO...) ends the current one
		if strings.HasPrefix(line, "0 ") {
			currentIndi, currentFam, currentSrc, srcLast = nil, nil, nil, nil
			lastWasNote, curCit = false, nil
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

		levelNum, _ := strconv.Atoi(level)
		if levelNum == 1 {
			curAlt = -1
		}
		if curCit != nil && levelNum <= citLevel {
			curCit = nil
		}

		if currentSrc != nil {
			switch tag {
			case "TITL":
				currentSrc.Title, srcLast = rest, &currentSrc.Title
			case "ABBR":
				if currentSrc.Title == "" {
					currentSrc.Title = rest
				}
			case "AUTH":
				currentSrc.Author, srcLast = rest, &currentSrc.Author
			case "PUBL":
				currentSrc.Publication, srcLast = rest, &currentSrc.Publication
			case "NOTE":
				if levelNum == 1 && !(strings.HasPrefix(rest, "@") && strings.HasSuffix(rest, "@")) {
					if currentSrc.Notes != "" {
						currentSrc.Notes += "\n"
					}
					currentSrc.Notes += rest
					srcLast = &currentSrc.Notes
				}
			case "CONC", "CONT":
				if srcLast != nil {
					if tag == "CONT" {
						*srcLast += "\n"
					}
					*srcLast += continuationValue(line)
				}
			}
		}

		if currentIndi != nil {
			p := currentIndi.Person
			switch tag {
			case "NAME":
				// Only the first NAME is the primary name; later ones are aliases
				if level == "1" && !nameSeen {
					nameSeen = true
					p.GivenName, p.Surname = splitGedcomName(m[3])
				} else if level == "1" {
					given, surname := splitGedcomName(m[3])
					currentIndi.AltNames = append(currentIndi.AltNames,
						store.AlternateName{NameType: "other", GivenName: given, Surname: surname})
					curAlt = len(currentIndi.AltNames) - 1
				}
			case "TYPE":
				if levelNum == 2 && curAlt >= 0 {
					currentIndi.AltNames[curAlt].NameType = altNameType(rest)
				}
			case "NICK":
				if rest != "" && (levelNum == 1 || levelNum == 2) {
					currentIndi.AltNames = append(currentIndi.AltNames,
						store.AlternateName{NameType: "nickname", GivenName: rest})
				}
			case "SOUR":
				if levelNum == 1 || (levelNum == 2 && (inBirth || inDeath)) {
					c := &ParsedCitation{}
					if strings.HasPrefix(rest, "@") && strings.HasSuffix(rest, "@") {
						c.SourceXref = strings.Trim(rest, "@")
					} else if rest != "" {
						c.InlineTitle = rest
					} else {
						break
					}
					if levelNum == 2 {
						c.Context = map[bool]string{true: "Birth", false: "Death"}[inBirth]
					}
					currentIndi.Citations = append(currentIndi.Citations, c)
					curCit, citLevel = c, levelNum
				}
			case "PAGE":
				if curCit != nil && levelNum == citLevel+1 {
					curCit.Detail = rest
				}
			case "QUAY":
				if curCit != nil && levelNum == citLevel+1 {
					curCit.Confidence = map[string]string{"0": "low", "1": "low", "2": "medium", "3": "high"}[rest]
				}
			case "TEXT":
				if curCit != nil && levelNum == citLevel+2 {
					if curCit.Transcription != "" {
						curCit.Transcription += "\n"
					}
					curCit.Transcription += rest
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
				if level == "1" && !(strings.HasPrefix(rest, "@") && strings.HasSuffix(rest, "@")) {
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
				if level == "1" {
					currentIndi.HasDeath = true
				}
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
	for xref, src := range result.Sources {
		if strings.TrimSpace(src.Title) == "" {
			src.Title = "Source " + xref
		}
	}
	for _, pp := range result.People {
		pp.Person.IsLiving = inferLiving(pp)
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

	var journal extrasJournal
	if _, err := writeExtras(s, parsed, idMap, nil, &journal); err != nil {
		return nil, err
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
