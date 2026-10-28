package dedupe

import (
	"testing"

	"genealogy/store"
)

func person(given, surname, gender, birth string) store.Person {
	return store.Person{GivenName: given, Surname: surname, Gender: gender, BirthDate: birth}
}

func TestParseDate(t *testing.T) {
	cases := map[string]Date{
		"3 DEC 1931":      {1931, 12, 3},
		"1931-12-03":      {1931, 12, 3},
		"ABT 1950":        {1950, 0, 0},
		"Mar 1900":        {1900, 3, 0},
		"BEF 12 May 1850": {1850, 5, 12},
		"unknown":         {},
		"":                {},
	}
	for in, want := range cases {
		if got := ParseDate(in); got != want {
			t.Errorf("ParseDate(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestDateScore(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"", "", 50, 50},
		{"1900", "", 20, 20},
		{"3 DEC 1931", "1931-12-03", 100, 100},
		{"3 DEC 1931", "10 DEC 1931", 80, 80},
		{"1931", "3 DEC 1931", 85, 85},
		{"1931", "1950", 10, 10},
	}
	for _, c := range cases {
		if got := DateScore(c.a, c.b); got < c.min || got > c.max {
			t.Errorf("DateScore(%q,%q) = %v, want %v-%v", c.a, c.b, got, c.min, c.max)
		}
	}
}

func TestNameScore(t *testing.T) {
	if got := NameScore(person("John", "Smith", "", ""), person("JOHN", "smith", "", "")); got != 100 {
		t.Errorf("case-insensitive exact = %v", got)
	}
	if got := NameScore(person("John", "Smith", "", ""), person("John William", "Smith", "", "")); got < 90 {
		t.Errorf("given-name prefix = %v, want >= 90", got)
	}
	if got := NameScore(person("John", "Smith", "", ""), person("Mary", "Jones", "", "")); got > 40 {
		t.Errorf("different names = %v, want low", got)
	}
}

func TestCompareAdjustments(t *testing.T) {
	a := person("John", "Smith", "M", "3 DEC 1931")
	same := Compare(a, person("John", "Smith", "M", "3 DEC 1931"))
	if same.Overall < 95 {
		t.Errorf("identical = %v", same.Overall)
	}
	if conflict := Compare(a, person("John", "Smith", "F", "3 DEC 1931")); conflict.Overall >= same.Overall/1.5 {
		t.Errorf("gender conflict not penalised: %v", conflict.Overall)
	}
	d1, d2 := a, person("John", "Smith", "M", "3 DEC 1931")
	d1.DeathDate, d2.DeathDate = "1990", "1850"
	if Compare(d1, d2).Overall >= same.Overall {
		t.Error("different death years not penalised")
	}
	u1, u2 := person("John", "Smith", "", ""), person("Johann", "Schmidt", "", "")
	u1.UID, u2.UID = "abc", "abc"
	if sc := Compare(u1, u2); !sc.SameUID || sc.Overall != 100 {
		t.Errorf("shared UID: %+v", sc)
	}
}

func TestSoundex(t *testing.T) {
	if soundex("smith") != soundex("smyth") {
		t.Error("smith/smyth should share a bucket")
	}
	if soundex("robert") != "R163" {
		t.Errorf("robert = %s", soundex("robert"))
	}
}

func TestFindDuplicates(t *testing.T) {
	people := []store.Person{
		person("John", "Smith", "M", "3 DEC 1931"),
		person("Mary", "Jones", "F", "1940"),
		person("John", "Smyth", "M", "3 DEC 1931"),
		person("John", "Smith", "M", "3 DEC 1931"),
	}
	pairs := FindDuplicates(people, DefaultThreshold)
	if len(pairs) != 3 {
		t.Fatalf("got %d pairs, want 3: %+v", len(pairs), pairs)
	}
	if pairs[0].Score.Overall < pairs[len(pairs)-1].Score.Overall {
		t.Error("pairs not sorted best first")
	}
}

func TestIndexMatch(t *testing.T) {
	existing := []store.Person{
		person("John", "Smith", "M", "3 DEC 1931"),
		person("Mary", "Jones", "F", "1940"),
	}
	idx := NewIndex(existing)

	got := idx.Match(person("John", "Smith", "M", "3 DEC 1931"), DefaultThreshold)
	if len(got) != 1 || got[0].Status != StatusExact {
		t.Fatalf("exact match: %+v", got)
	}
	got = idx.Match(person("John", "Smyth", "M", "1931"), DefaultThreshold)
	if len(got) != 1 || got[0].Status != StatusLikely {
		t.Fatalf("likely match: %+v", got)
	}
	if got := idx.Match(person("Peter", "Brown", "M", "1800"), DefaultThreshold); len(got) != 0 {
		t.Fatalf("expected new person, got %+v", got)
	}
}
