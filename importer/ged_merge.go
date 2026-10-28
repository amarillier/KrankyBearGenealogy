package importer

import (
	"fmt"
	"strings"
	"time"

	"genealogy/store"
)

// GEDCOM merge: apply a parsed GEDCOM file to a database that already has data,
// following a per-person plan (add, merge into an existing person, or skip).

// MergeAction is what to do with one incoming person.
type MergeAction int

const (
	ActionAdd   MergeAction = iota // create as a new person (the default)
	ActionMerge                    // fold into an existing person
	ActionSkip                     // ignore this person and any family links to them
)

// FieldSource picks which side's value wins for a conflicting field.
type FieldSource int

const (
	UseExisting FieldSource = iota
	UseIncoming
	UseBoth // notes only: keep both texts
)

// Merge field keys, used in FieldDiff and PersonPlan.Choices.
const (
	FieldGivenName  = "given_name"
	FieldSurname    = "surname"
	FieldGender     = "gender"
	FieldBirthDate  = "birth_date"
	FieldBirthPlace = "birth_place"
	FieldDeathDate  = "death_date"
	FieldDeathPlace = "death_place"
	FieldUID        = "uid"
	FieldNotes      = "notes"
)

type mergeField struct {
	key, label string
	get        func(*store.Person) *string
}

var mergeFields = []mergeField{
	{FieldGivenName, "Given Name", func(p *store.Person) *string { return &p.GivenName }},
	{FieldSurname, "Surname", func(p *store.Person) *string { return &p.Surname }},
	{FieldGender, "Gender", func(p *store.Person) *string { return &p.Gender }},
	{FieldBirthDate, "Birth Date", func(p *store.Person) *string { return &p.BirthDate }},
	{FieldBirthPlace, "Birth Place", func(p *store.Person) *string { return &p.BirthPlace }},
	{FieldDeathDate, "Death Date", func(p *store.Person) *string { return &p.DeathDate }},
	{FieldDeathPlace, "Death Place", func(p *store.Person) *string { return &p.DeathPlace }},
	{FieldUID, "UID", func(p *store.Person) *string { return &p.UID }},
	{FieldNotes, "Notes", func(p *store.Person) *string { return &p.Notes }},
}

// FieldDiff describes one field where the existing and incoming person differ.
type FieldDiff struct {
	Key, Label         string
	Existing, Incoming string
	// Conflict is true when both sides have a different non-empty value, so a
	// person has to choose. Otherwise the non-empty side is used automatically.
	Conflict bool
}

// DiffPersons lists the mergeable fields where existing and incoming differ.
func DiffPersons(existing, incoming store.Person) []FieldDiff {
	var out []FieldDiff
	for _, f := range mergeFields {
		e := strings.TrimSpace(*f.get(&existing))
		i := strings.TrimSpace(*f.get(&incoming))
		if strings.EqualFold(e, i) || i == "" && e == "" {
			continue
		}
		out = append(out, FieldDiff{
			Key: f.key, Label: f.label, Existing: e, Incoming: i,
			Conflict: e != "" && i != "",
		})
	}
	return out
}

// MergePersons returns existing with incoming's data folded in. A field only
// the incoming side has is filled in; conflicting fields keep the existing value
// unless choices says otherwise. Fields present in choices are always honoured.
func MergePersons(existing, incoming store.Person, choices map[string]FieldSource) store.Person {
	merged := existing
	for _, f := range mergeFields {
		e := *f.get(&existing)
		i := *f.get(&incoming)
		choice, explicit := choices[f.key]
		if !explicit {
			if strings.TrimSpace(e) == "" {
				choice = UseIncoming
			} else {
				choice = UseExisting
			}
		}
		dst := f.get(&merged)
		switch choice {
		case UseIncoming:
			if strings.TrimSpace(i) != "" || explicit {
				*dst = i
			}
		case UseBoth:
			*dst = combineText(e, i)
		}
	}
	if strings.TrimSpace(merged.DeathDate) != "" {
		merged.IsLiving = false
	}
	return merged
}

func combineText(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	switch {
	case a == "":
		return b
	case b == "" || strings.Contains(a, b):
		return a
	case strings.Contains(b, a):
		return b
	}
	return a + "\n\n" + b
}

// PersonPlan is the decision for one incoming person.
type PersonPlan struct {
	Action     MergeAction
	ExistingID int64                  // for ActionMerge
	Choices    map[string]FieldSource // field key -> winner, for ActionMerge
}

// MergePlan maps GEDCOM xrefs (as in ParsedPerson.GedID) to decisions. People
// with no entry are added as new.
type MergePlan map[string]PersonPlan

// MergeResult summarises what a merge did.
type MergeResult struct {
	Added, Merged, Skipped int
	RelationshipsAdded     int
	RelationshipsExisting  int // links already present, left untouched
	SourcesAdded           int
	CitationsAdded         int
	AltNamesAdded          int
}

func (r MergeResult) String() string {
	out := fmt.Sprintf("%d added, %d merged, %d skipped; %d relationships added (%d already existed)",
		r.Added, r.Merged, r.Skipped, r.RelationshipsAdded, r.RelationshipsExisting)
	if r.CitationsAdded+r.SourcesAdded+r.AltNamesAdded > 0 {
		out += fmt.Sprintf("; %d sources, %d citations, %d alternate names added",
			r.SourcesAdded, r.CitationsAdded, r.AltNamesAdded)
	}
	return out
}

type relKey struct {
	subject, object int64
	relType         string
}

// ApplyMerge writes parsed into s following plan. The store has no multi-call
// transaction, so every change is journaled and undone if anything fails; on
// success a summary entry is added to the audit log.
func ApplyMerge(s *store.Store, parsed *ParsedGedcom, plan MergePlan) (MergeResult, error) {
	var res MergeResult
	var createdPeople []int64
	var createdRels []relKey
	var updatedBefore []store.Person
	var journal extrasJournal
	nameAsAlt := map[string]bool{}

	rollback := func(cause error) (MergeResult, error) {
		journal.rollback(s)
		for _, k := range createdRels {
			_ = s.DeleteRelationship(k.subject, k.object, k.relType)
			_ = s.DeleteRelationship(k.object, k.subject, k.relType)
		}
		for _, id := range createdPeople {
			_ = s.DeletePerson(id)
		}
		for i := range updatedBefore {
			_ = s.UpdatePerson(&updatedBefore[i])
		}
		return MergeResult{}, fmt.Errorf("merge failed and was rolled back: %w", cause)
	}

	existing, err := s.GetPeople()
	if err != nil {
		return MergeResult{}, err
	}
	byID := make(map[int64]store.Person, len(existing))
	for _, p := range existing {
		byID[p.ID] = p
	}

	idMap := map[string]int64{}
	for _, pp := range parsed.People {
		pl := plan[pp.GedID]
		switch pl.Action {
		case ActionSkip:
			res.Skipped++
		case ActionMerge:
			cur, ok := byID[pl.ExistingID]
			if !ok {
				return rollback(fmt.Errorf("person %s: existing person %d not found", pp.GedID, pl.ExistingID))
			}
			merged := MergePersons(cur, *pp.Person, pl.Choices)
			if merged != cur {
				updatedBefore = append(updatedBefore, cur)
				if err := s.UpdatePerson(&merged); err != nil {
					return rollback(fmt.Errorf("update person %s: %w", pp.GedID, err))
				}
			}
			idMap[pp.GedID] = cur.ID
			nameAsAlt[pp.GedID] = true
			res.Merged++
		default:
			p := *pp.Person
			if err := s.CreatePerson(&p); err != nil {
				return rollback(fmt.Errorf("create person %s: %w", pp.GedID, err))
			}
			createdPeople = append(createdPeople, p.ID)
			idMap[pp.GedID] = p.ID
			res.Added++
		}
	}

	extras, err := writeExtras(s, parsed, idMap, nameAsAlt, &journal)
	if err != nil {
		return rollback(err)
	}
	res.SourcesAdded, res.CitationsAdded, res.AltNamesAdded = extras.Sources, extras.Citations, extras.AltNames

	addRel := func(rel *store.Relationship) error {
		if rel.SubjectID == rel.ObjectID {
			return nil
		}
		found, err := s.GetRelationshipsBetween(rel.SubjectID, rel.ObjectID, rel.Type)
		if err != nil {
			return err
		}
		if len(found) > 0 {
			res.RelationshipsExisting++
			cur := found[0]
			// Fill marriage details the existing link lacks
			if rel.Type == "spouse" && (cur.MarriageDate == "" && rel.MarriageDate != "" ||
				cur.MarriagePlace == "" && rel.MarriagePlace != "" ||
				cur.DivorceDate == "" && rel.DivorceDate != "") {
				if cur.MarriageDate == "" {
					cur.MarriageDate = rel.MarriageDate
				}
				if cur.MarriagePlace == "" {
					cur.MarriagePlace = rel.MarriagePlace
				}
				if cur.DivorceDate == "" {
					cur.DivorceDate = rel.DivorceDate
					if cur.EndReason == "" {
						cur.EndReason = rel.EndReason
					}
				}
				return s.UpdateRelationship(&cur)
			}
			return nil
		}
		if err := s.CreateRelationship(rel); err != nil {
			return err
		}
		createdRels = append(createdRels, relKey{rel.SubjectID, rel.ObjectID, rel.Type})
		res.RelationshipsAdded++
		return nil
	}

	for _, fam := range parsed.Families {
		h, hok := idMap[fam.Husband]
		w, wok := idMap[fam.Wife]
		if hok && wok {
			rel := &store.Relationship{
				SubjectID: h, ObjectID: w, Type: "spouse",
				MarriageDate: fam.MarriageDate, MarriagePlace: fam.MarriagePlace, DivorceDate: fam.DivorceDate,
			}
			if fam.DivorceDate != "" {
				rel.EndReason = "divorce"
			}
			if err := addRel(rel); err != nil {
				return rollback(fmt.Errorf("family %s spouse link: %w", fam.GedID, err))
			}
		}
		for _, child := range fam.Children {
			cid, ok := idMap[child]
			if !ok {
				continue
			}
			for _, parent := range []int64{h, w} {
				if parent == 0 {
					continue
				}
				if err := addRel(&store.Relationship{SubjectID: parent, ObjectID: cid, Type: "child"}); err != nil {
					return rollback(fmt.Errorf("family %s child link: %w", fam.GedID, err))
				}
			}
		}
	}

	_ = s.CreateAuditLogEntry(&store.AuditLogEntry{
		Timestamp:     time.Now(),
		OperationType: "gedcom_merge",
		Description:   "GEDCOM merge: " + res.String(),
	})
	return res, nil
}
