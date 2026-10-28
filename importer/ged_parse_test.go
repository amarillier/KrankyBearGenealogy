package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
