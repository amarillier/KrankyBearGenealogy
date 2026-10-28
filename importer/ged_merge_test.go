package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"genealogy/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMergePersonsDefaults(t *testing.T) {
	existing := store.Person{GivenName: "John", Surname: "Smith", BirthDate: "1931", Notes: "old"}
	incoming := store.Person{GivenName: "Johnny", Surname: "Smith", BirthDate: "3 DEC 1931",
		BirthPlace: "Durban", DeathDate: "2001", IsLiving: false, Notes: "new"}
	incoming.Gender = "M"

	got := MergePersons(existing, incoming, nil)
	if got.GivenName != "John" || got.BirthDate != "1931" || got.Notes != "old" {
		t.Errorf("conflicts should keep existing: %+v", got)
	}
	if got.BirthPlace != "Durban" || got.Gender != "M" || got.DeathDate != "2001" {
		t.Errorf("blanks should be filled: %+v", got)
	}

	got = MergePersons(existing, incoming, map[string]FieldSource{
		FieldBirthDate: UseIncoming, FieldNotes: UseBoth,
	})
	if got.BirthDate != "3 DEC 1931" || got.Notes != "old\n\nnew" {
		t.Errorf("explicit choices ignored: %+v", got)
	}
}

func TestDiffPersons(t *testing.T) {
	diffs := DiffPersons(
		store.Person{GivenName: "John", Surname: "Smith", BirthDate: "1931"},
		store.Person{GivenName: "JOHN", Surname: "Smith", BirthDate: "1932", BirthPlace: "Durban"})
	byKey := map[string]FieldDiff{}
	for _, d := range diffs {
		byKey[d.Key] = d
	}
	if len(diffs) != 2 || !byKey[FieldBirthDate].Conflict || byKey[FieldBirthPlace].Conflict {
		t.Errorf("diffs = %+v", diffs)
	}
}

func TestApplyMerge(t *testing.T) {
	s := newTestStore(t)
	john := &store.Person{GivenName: "John", Surname: "van der Berg", Gender: "M", BirthDate: "3 DEC 1931"}
	if err := s.CreatePerson(john); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseGedcomReader(strings.NewReader(sampleGed))
	if err != nil {
		t.Fatal(err)
	}
	plan := MergePlan{
		"I1": {Action: ActionMerge, ExistingID: john.ID},
		"I3": {Action: ActionSkip},
	}
	res, err := ApplyMerge(s, parsed, plan)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Merged != 1 || res.Skipped != 1 || res.RelationshipsAdded != 1 {
		t.Errorf("result = %s", res)
	}

	people, _ := s.GetPeople()
	if len(people) != 2 {
		t.Fatalf("%d people, want 2 (John + Mary)", len(people))
	}
	got, _ := s.GetPersonByID(john.ID)
	if got.BirthPlace != "Durban, South Africa" || got.DeathDate != "1 JAN 2001" {
		t.Errorf("John not enriched: %+v", got)
	}
	rels, _ := s.GetRelationshipsBetween(john.ID, otherID(people, john.ID), "spouse")
	if len(rels) != 1 || rels[0].MarriagePlace != "London" {
		t.Errorf("spouse link: %+v", rels)
	}

	// Applying the same merge again must not duplicate links
	res, err = ApplyMerge(s, parsed, MergePlan{
		"I1": {Action: ActionMerge, ExistingID: john.ID},
		"I2": {Action: ActionMerge, ExistingID: otherID(people, john.ID)},
		"I3": {Action: ActionSkip},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RelationshipsAdded != 0 || res.RelationshipsExisting != 1 {
		t.Errorf("second merge result = %s", res)
	}
}

func TestApplyMergeRollback(t *testing.T) {
	s := newTestStore(t)
	parsed, err := ParseGedcomReader(strings.NewReader(sampleGed))
	if err != nil {
		t.Fatal(err)
	}
	// I1 and I2 are created first, then I3's merge target turns out not to exist
	_, err = ApplyMerge(s, parsed, MergePlan{"I3": {Action: ActionMerge, ExistingID: 999}})
	if err == nil {
		t.Fatal("expected error")
	}
	if people, _ := s.GetPeople(); len(people) != 0 {
		t.Errorf("rollback left %d people", len(people))
	}
}

func otherID(people []store.Person, not int64) int64 {
	for _, p := range people {
		if p.ID != not {
			return p.ID
		}
	}
	return 0
}

func TestApplyMergeSourcesAndAltNames(t *testing.T) {
	s := newTestStore(t)
	john := &store.Person{GivenName: "Jon", Surname: "Smith", BirthDate: "1880"}
	if err := s.CreatePerson(john); err != nil {
		t.Fatal(err)
	}
	// The same census source already exists in the database
	if err := s.CreateSource(&store.Source{Title: "1900 US Census,\nDurban County", Author: "us census bureau"}); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseGedcomReader(strings.NewReader(sourcedGed))
	if err != nil {
		t.Fatal(err)
	}
	plan := MergePlan{"I1": {Action: ActionMerge, ExistingID: john.ID}, "I2": {Action: ActionSkip}}
	res, err := ApplyMerge(s, parsed, plan)
	if err != nil {
		t.Fatal(err)
	}
	if res.SourcesAdded != 1 || res.CitationsAdded != 3 || res.AltNamesAdded != 4 {
		t.Errorf("result = %s", res) // 3 alt names + the incoming primary name "John Smith"
	}
	sources, _ := s.GetAllSources()
	if len(sources) != 2 {
		t.Errorf("%d sources, want 2 (existing census reused + Family Bible)", len(sources))
	}

	// Merging the same file again adds nothing
	res, err = ApplyMerge(s, parsed, plan)
	if err != nil {
		t.Fatal(err)
	}
	if res.SourcesAdded+res.CitationsAdded+res.AltNamesAdded != 0 {
		t.Errorf("second merge result = %s", res)
	}
}

func TestExtrasJournalRollback(t *testing.T) {
	s := newTestStore(t)
	p := &store.Person{GivenName: "John", Surname: "Smith"}
	if err := s.CreatePerson(p); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseGedcomReader(strings.NewReader(sourcedGed))
	if err != nil {
		t.Fatal(err)
	}
	var j extrasJournal
	if _, err := writeExtras(s, parsed, map[string]int64{"I1": p.ID}, nil, &j); err != nil {
		t.Fatal(err)
	}
	j.rollback(s)
	sources, _ := s.GetAllSources()
	cites, _ := s.GetCitationsForPerson(p.ID)
	alts, _ := s.GetAlternateNames(p.ID)
	if len(sources)+len(cites)+len(alts) != 0 {
		t.Errorf("rollback left %d sources, %d citations, %d alternate names", len(sources), len(cites), len(alts))
	}
}
