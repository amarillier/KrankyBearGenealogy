package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genealogy/store"
)

const sampleGed = "\xef\xbb\xbf0 HEAD\n" +
	"1 CHAR UTF-8\n" +
	"0 @I1@ INDI\n" +
	"1 NAME John /van der Berg/ Jr\n" +
	"2 GIVN John\n" +
	"1 NAME Johnny /Berg/\n" +
	"1 SEX M\n" +
	"1 BIRT\n" +
	"2 DATE 3 DEC 1931\n" +
	"2 PLAC Durban, South Africa\n" +
	"2 NOTE ignored birth note\n" +
	"1 DEAT\n" +
	"2 DATE 1 JAN 2001\n" +
	"1 NOTE First line\n" +
	"2 CONC  continued\n" +
	"2 CONT second line\n" +
	"0 @I2@ INDI\n" +
	"1 NAME Mary Smith\n" +
	"1 SEX F\n" +
	"0 @I3@ INDI\n" +
	"1 NAME Kid /Berg/\n" +
	"0 @F1@ FAM\n" +
	"1 HUSB @I1@\n" +
	"1 WIFE @I2@\n" +
	"1 CHIL @I3@\n" +
	"1 MARR\n" +
	"2 DATE 5 MAY 1955\n" +
	"2 PLAC London\n" +
	"1 DIV\n" +
	"2 DATE 1 JUN 1970\n" +
	"0 TRLR\n"

func TestParseGedcomReader(t *testing.T) {
	g, err := ParseGedcomReader(strings.NewReader(sampleGed))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.People) != 3 || len(g.Families) != 1 {
		t.Fatalf("got %d people, %d families", len(g.People), len(g.Families))
	}

	p := g.People[0].Person
	if g.People[0].GedID != "I1" || p.GivenName != "John Jr" || p.Surname != "van der Berg" {
		t.Errorf("name = %q / %q (id %s)", p.GivenName, p.Surname, g.People[0].GedID)
	}
	if p.Gender != "M" || p.BirthDate != "3 DEC 1931" || p.BirthPlace != "Durban, South Africa" || p.DeathDate != "1 JAN 2001" {
		t.Errorf("unexpected person fields: %+v", p)
	}
	if want := "First line continued\nsecond line"; p.Notes != want {
		t.Errorf("notes = %q, want %q", p.Notes, want)
	}

	m := g.People[1].Person
	if m.GivenName != "Mary" || m.Surname != "Smith" {
		t.Errorf("slashless name = %q / %q", m.GivenName, m.Surname)
	}

	f := g.Families[0]
	if f.Husband != "I1" || f.Wife != "I2" || len(f.Children) != 1 || f.Children[0] != "I3" {
		t.Errorf("family = %+v", f)
	}
	if f.MarriageDate != "5 MAY 1955" || f.MarriagePlace != "London" || f.DivorceDate != "1 JUN 1970" {
		t.Errorf("marriage = %+v", f)
	}
}

func TestSplitGedcomName(t *testing.T) {
	cases := []struct{ in, given, surname string }{
		{"John /Smith/", "John", "Smith"},
		{"/Smith/", "", "Smith"},
		{"Mary Ann /de la Cruz/ III", "Mary Ann III", "de la Cruz"},
		{"John Smith", "John", "Smith"},
		{"Cher", "Cher", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		g, s := splitGedcomName(c.in)
		if g != c.given || s != c.surname {
			t.Errorf("splitGedcomName(%q) = %q, %q; want %q, %q", c.in, g, s, c.given, c.surname)
		}
	}
}

func TestParseSampleFiles(t *testing.T) {
	for _, name := range []string{"royal92.ged", "bronte.ged", "shakespeare.ged"} {
		g, err := ParseGedcom(filepath.Join("..", "sampledata", name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(g.People) == 0 || len(g.Families) == 0 {
			t.Errorf("%s: parsed %d people, %d families", name, len(g.People), len(g.Families))
		}
	}
}

func TestImportRoundTrip(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "in.ged")
	if err := writeFile(path, sampleGed); err != nil {
		t.Fatal(err)
	}
	if err := Import(path, s); err != nil {
		t.Fatal(err)
	}
	people, err := s.GetPeople()
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 3 {
		t.Fatalf("stored %d people, want 3", len(people))
	}
	for _, p := range people {
		if p.GivenName == "John Jr" {
			rels, _ := s.GetRelationshipsForPerson(p.ID)
			types := map[string]bool{}
			for _, r := range rels {
				types[r.Type] = true
			}
			if !types["spouse"] || !types["child"] {
				t.Errorf("John's relationship types = %v, want spouse and child", types)
			}
			return
		}
	}
	t.Error("John not found after import")
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestInferLiving(t *testing.T) {
	old := now
	now = func() time.Time { return time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) }
	defer func() { now = old }()

	ged := "0 @I1@ INDI\n1 NAME Young /One/\n1 BIRT\n2 DATE 12 MAR 1980\n" +
		"0 @I2@ INDI\n1 NAME Old /Two/\n1 BIRT\n2 DATE 1850\n" +
		"0 @I3@ INDI\n1 NAME Dead /Three/\n1 BIRT\n2 DATE 1960\n1 DEAT Y\n" +
		"0 @I4@ INDI\n1 NAME Dated /Four/\n1 BIRT\n2 DATE 1960\n1 DEAT\n2 DATE 2010\n" +
		"0 @I5@ INDI\n1 NAME Undated /Five/\n" +
		"0 @I6@ INDI\n1 NAME Edge /Six/\n1 BIRT\n2 DATE ABT 1927\n"
	g, err := ParseGedcomReader(strings.NewReader(ged))
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, false, false, false, true}
	for i, pp := range g.People {
		if pp.Person.IsLiving != want[i] {
			t.Errorf("%s (%s): IsLiving = %v, want %v", pp.GedID, pp.Person.Surname, pp.Person.IsLiving, want[i])
		}
	}
}

const sourcedGed = "0 HEAD\n" +
	"0 @S1@ SOUR\n" +
	"1 TITL 1900 US Census,\n" +
	"2 CONT Durban County\n" +
	"1 AUTH US Census Bureau\n" +
	"0 @N1@ NOTE\n" +
	"1 CONT a shared note\n" +
	"0 @I1@ INDI\n" +
	"1 NAME John /Smith/\n" +
	"1 NAME Johnny /Smythe/\n" +
	"2 TYPE aka\n" +
	"1 NAME Joan /Doe/\n" +
	"2 TYPE maiden\n" +
	"1 NICK Jack\n" +
	"1 NOTE @N1@\n" +
	"1 SOUR @S1@\n" +
	"2 PAGE sheet 4, line 12\n" +
	"2 QUAY 3\n" +
	"2 DATA\n" +
	"3 TEXT John Smith, age 20\n" +
	"1 BIRT\n" +
	"2 DATE 1880\n" +
	"2 SOUR @S1@\n" +
	"3 PAGE sheet 9\n" +
	"1 SOUR Family Bible\n" +
	"0 @I2@ INDI\n" +
	"1 NAME Mary /Jones/\n" +
	"1 SOUR @S1@\n" +
	"2 PAGE sheet 4, line 13\n" +
	"0 TRLR\n"

func TestParseSourcesAndAltNames(t *testing.T) {
	g, err := ParseGedcomReader(strings.NewReader(sourcedGed))
	if err != nil {
		t.Fatal(err)
	}
	src := g.Sources["S1"]
	if src == nil || src.Title != "1900 US Census,\nDurban County" || src.Author != "US Census Bureau" {
		t.Fatalf("source = %+v", src)
	}
	john := g.People[0]
	if john.Person.GivenName != "John" || john.Person.Notes != "" {
		t.Errorf("primary name / pointer note leaked: %+v", john.Person)
	}
	types := map[string]string{}
	for _, a := range john.AltNames {
		types[a.GivenName+" "+a.Surname] = a.NameType
	}
	want := map[string]string{"Johnny Smythe": "other", "Joan Doe": "maiden", "Jack ": "nickname"}
	for k, v := range want {
		if types[k] != v {
			t.Errorf("alt name %q type = %q, want %q (all: %v)", k, types[k], v, types)
		}
	}
	if len(john.Citations) != 3 {
		t.Fatalf("citations = %d, want 3", len(john.Citations))
	}
	c := john.Citations[0]
	if c.SourceXref != "S1" || c.Detail != "sheet 4, line 12" || c.Confidence != "high" || c.Transcription != "John Smith, age 20" {
		t.Errorf("citation 0 = %+v", c)
	}
	if b := john.Citations[1]; b.Context != "Birth" || b.Detail != "sheet 9" {
		t.Errorf("birth citation = %+v", b)
	}
	if in := john.Citations[2]; in.InlineTitle != "Family Bible" {
		t.Errorf("inline citation = %+v", in)
	}
	// The NOTE record after the source must not leak into the next person
	if g.People[1].Person.Notes != "" || len(g.People[1].Citations) != 1 {
		t.Errorf("second person = %+v", g.People[1])
	}
}

func TestImportStoresSourcesAndAltNames(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(t.TempDir(), "s.ged")
	if err := writeFile(path, sourcedGed); err != nil {
		t.Fatal(err)
	}
	if err := Import(path, s); err != nil {
		t.Fatal(err)
	}
	sources, _ := s.GetAllSources()
	if len(sources) != 2 { // census + inline Family Bible
		t.Errorf("%d sources, want 2", len(sources))
	}
	people, _ := s.GetPeople()
	for _, p := range people {
		if p.GivenName != "John" {
			continue
		}
		cites, _ := s.GetCitationsForPerson(p.ID)
		alts, _ := s.GetAlternateNames(p.ID)
		if len(cites) != 3 || len(alts) != 3 {
			t.Errorf("John has %d citations, %d alternate names; want 3 and 3", len(cites), len(alts))
		}
		return
	}
	t.Error("John not found")
}
