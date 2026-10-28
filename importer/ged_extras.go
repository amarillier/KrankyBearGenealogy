package importer

import (
	"fmt"
	"strings"

	"genealogy/store"
)

// extrasJournal records what writeExtras created so a failed merge can remove it.
type extrasJournal struct {
	sourceIDs, citationIDs, altNameIDs []int64
}

func (j *extrasJournal) rollback(s *store.Store) {
	for _, id := range j.citationIDs {
		_ = s.DeleteCitation(id)
	}
	for _, id := range j.altNameIDs {
		_ = s.DeleteAlternateName(id)
	}
	for _, id := range j.sourceIDs {
		_ = s.DeleteSource(id)
	}
}

// extrasCount is what writeExtras added.
type extrasCount struct {
	Sources, Citations, AltNames int
}

func sameText(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// writeExtras stores the sources, citations and alternate names from parsed for
// every person in idMap (GEDCOM xref -> database ID). A source that matches an
// existing one by title and author is reused, and a citation or alternate name
// the person already has is skipped, so merging the same file twice adds nothing.
// For xrefs in nameAsAlt, the incoming primary name is also kept as an alternate
// name when it differs from the person's current name.
func writeExtras(s *store.Store, parsed *ParsedGedcom, idMap map[string]int64, nameAsAlt map[string]bool, j *extrasJournal) (extrasCount, error) {
	var n extrasCount

	existingSources, err := s.GetAllSources()
	if err != nil {
		return n, err
	}
	sourceIDs := map[string]int64{} // GEDCOM xref or "inline:<title>" -> database source ID
	resolve := func(title string, build func() *store.Source) (int64, error) {
		for _, es := range existingSources {
			if sameText(es.Title, title) {
				if cand := build(); sameText(es.Author, cand.Author) {
					return es.ID, nil
				}
			}
		}
		src := build()
		if err := s.CreateSource(src); err != nil {
			return 0, err
		}
		j.sourceIDs = append(j.sourceIDs, src.ID)
		existingSources = append(existingSources, *src)
		n.Sources++
		return src.ID, nil
	}
	sourceFor := func(c *ParsedCitation) (int64, error) {
		key := "inline:" + strings.ToLower(c.InlineTitle)
		if c.SourceXref != "" {
			key = c.SourceXref
		}
		if id, ok := sourceIDs[key]; ok {
			return id, nil
		}
		var id int64
		var err error
		if c.SourceXref != "" {
			src, ok := parsed.Sources[c.SourceXref]
			if !ok {
				return 0, nil // dangling reference: nothing to cite
			}
			id, err = resolve(src.Title, func() *store.Source { cp := *src; return &cp })
		} else {
			id, err = resolve(c.InlineTitle, func() *store.Source {
				return &store.Source{Title: c.InlineTitle, SourceType: "other"}
			})
		}
		if err == nil {
			sourceIDs[key] = id
		}
		return id, err
	}

	for _, pp := range parsed.People {
		pid, ok := idMap[pp.GedID]
		if !ok {
			continue
		}
		wantAlt := nameAsAlt[pp.GedID]
		if len(pp.AltNames) == 0 && len(pp.Citations) == 0 && !wantAlt {
			continue
		}
		person, err := s.GetPersonByID(pid)
		if err != nil {
			return n, fmt.Errorf("person %s: %w", pp.GedID, err)
		}

		alts := append([]store.AlternateName(nil), pp.AltNames...)
		if wantAlt {
			alts = append(alts, store.AlternateName{NameType: "spelling",
				GivenName: pp.Person.GivenName, Surname: pp.Person.Surname,
				Notes: "Name in merged GEDCOM file"})
		}
		have, err := s.GetAlternateNames(pid)
		if err != nil {
			return n, err
		}
		for _, alt := range alts {
			if strings.TrimSpace(alt.GivenName+alt.Surname) == "" {
				continue
			}
			if sameText(alt.GivenName, person.GivenName) && sameText(alt.Surname, person.Surname) {
				continue
			}
			dup := false
			for _, h := range have {
				if sameText(h.GivenName, alt.GivenName) && sameText(h.Surname, alt.Surname) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			alt.PersonID = pid
			if err := s.CreateAlternateName(&alt); err != nil {
				return n, fmt.Errorf("alternate name for %s: %w", pp.GedID, err)
			}
			j.altNameIDs = append(j.altNameIDs, alt.ID)
			have = append(have, alt)
			n.AltNames++
		}

		if len(pp.Citations) == 0 {
			continue
		}
		haveCites, err := s.GetCitationsForPerson(pid)
		if err != nil {
			return n, err
		}
		for _, c := range pp.Citations {
			sid, err := sourceFor(c)
			if err != nil {
				return n, fmt.Errorf("source for %s: %w", pp.GedID, err)
			}
			if sid == 0 {
				continue
			}
			dup := false
			for _, h := range haveCites {
				if h.SourceID == sid && sameText(h.CitationDetail, c.Detail) && sameText(h.Transcription, c.Transcription) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			cit := &store.Citation{SourceID: sid, PersonID: pid, CitationDetail: c.Detail,
				Transcription: c.Transcription, Confidence: c.Confidence}
			if c.Context != "" {
				cit.Notes = "Cited for: " + c.Context
			}
			if err := s.CreateCitation(cit); err != nil {
				return n, fmt.Errorf("citation for %s: %w", pp.GedID, err)
			}
			j.citationIDs = append(j.citationIDs, cit.ID)
			haveCites = append(haveCites, *cit)
			n.Citations++
		}
	}
	return n, nil
}
