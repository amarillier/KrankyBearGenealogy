package store

import (
	"path/filepath"
	"testing"
)

func newMergeTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
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

func mustPerson(t *testing.T, s *Store, given, surname string) *Person {
	t.Helper()
	p := &Person{GivenName: given, Surname: surname}
	if err := s.CreatePerson(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func count(t *testing.T, s *Store, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMergePersonRecords(t *testing.T) {
	s := newMergeTestStore(t)
	keep := mustPerson(t, s, "John", "Smith")
	dup := mustPerson(t, s, "Jon", "Smyth")
	dad := mustPerson(t, s, "Bill", "Smith")
	wife := mustPerson(t, s, "Mary", "Jones")

	// dad is parent of both records; wife is spouse of the duplicate only
	for _, p := range []*Person{keep, dup} {
		if err := s.CreateRelationship(&Relationship{SubjectID: dad.ID, ObjectID: p.ID, Type: "child"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateRelationship(&Relationship{SubjectID: dup.ID, ObjectID: wife.ID, Type: "spouse", MarriageDate: "1955", MarriagePlace: "London"}); err != nil {
		t.Fatal(err)
	}
	// the two records are also (wrongly) linked to each other
	if err := s.CreateRelationship(&Relationship{SubjectID: keep.ID, ObjectID: dup.ID, Type: "sibling"}); err != nil {
		t.Fatal(err)
	}

	if err := s.CreateEvent(&Event{PersonID: dup.ID, Type: "OCCU", Place: "Durban"}); err != nil {
		t.Fatal(err)
	}
	src := &Source{Title: "Census 1900"}
	if err := s.CreateSource(src); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCitation(&Citation{SourceID: src.ID, PersonID: dup.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTodo(&ResearchTodo{PersonID: dup.ID, Description: "find baptism", Priority: "low", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	m := &Media{Title: "photo", MediaType: "image"}
	if err := s.CreateMedia(m); err != nil {
		t.Fatal(err)
	}
	s.LinkMediaToPerson(m.ID, dup.ID)
	s.LinkMediaToPerson(m.ID, keep.ID) // already linked to both: must not violate UNIQUE

	merged := *keep
	merged.BirthPlace = "Durban"
	if err := s.MergePersonRecords(keep.ID, dup.ID, merged); err != nil {
		t.Fatal(err)
	}

	if count(t, s, `SELECT COUNT(*) FROM persons WHERE id=?`, dup.ID) != 0 {
		t.Error("duplicate not deleted")
	}
	got, _ := s.GetPersonByID(keep.ID)
	if got.BirthPlace != "Durban" {
		t.Errorf("merged fields not saved: %+v", got)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM relationships WHERE subject_id=? AND object_id=? AND type='child'`, dad.ID, keep.ID); n != 1 {
		t.Errorf("dad->keep child links = %d, want 1 (no duplicate)", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM relationships WHERE subject_id=object_id`); n != 0 {
		t.Errorf("%d self relationships", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM relationships WHERE type='sibling'`); n != 0 {
		t.Errorf("self-sibling links remain: %d", n)
	}
	spouse, _ := s.GetRelationshipsBetween(keep.ID, wife.ID, "spouse")
	if len(spouse) != 1 || spouse[0].MarriagePlace != "London" {
		t.Errorf("spouse link not transferred: %+v", spouse)
	}
	back, _ := s.GetRelationshipsBetween(wife.ID, keep.ID, "spouse")
	if len(back) != 1 {
		t.Errorf("inverse spouse link missing: %+v", back)
	}
	for table, want := range map[string]int{"events": 1, "citations": 1, "research_todos": 1, "person_media": 1} {
		if n := count(t, s, `SELECT COUNT(*) FROM `+table+` WHERE person_id=?`, keep.ID); n != want {
			t.Errorf("%s for kept person = %d, want %d", table, n, want)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM alternate_names WHERE person_id=? AND given_name='Jon' AND surname='Smyth'`, keep.ID); n != 1 {
		t.Errorf("duplicate's name not kept as alternate name (%d)", n)
	}
}

func TestMergePersonRecordsErrors(t *testing.T) {
	s := newMergeTestStore(t)
	a := mustPerson(t, s, "A", "A")
	if err := s.MergePersonRecords(a.ID, a.ID, *a); err == nil {
		t.Error("merging a person into themselves should fail")
	}
	if err := s.MergePersonRecords(a.ID, 999, *a); err == nil {
		t.Error("missing duplicate should fail")
	}
	if n := count(t, s, `SELECT COUNT(*) FROM persons`); n != 1 {
		t.Errorf("%d people left", n)
	}
}
