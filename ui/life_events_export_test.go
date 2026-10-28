package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"genealogy/store"
)

func newExportTestStore(t *testing.T) *store.Store {
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

func TestWebsiteLifeEvents(t *testing.T) {
	s := newExportTestStore(t)
	dead := &store.Person{GivenName: "Anna", Surname: "Berg", BirthDate: "1880", DeathDate: "1950"}
	alive := &store.Person{GivenName: "Carl", Surname: "Berg", BirthDate: "1990", IsLiving: true}
	for _, p := range []*store.Person{dead, alive} {
		if err := s.CreatePerson(p); err != nil {
			t.Fatal(err)
		}
	}
	s.CreateEvent(&store.Event{PersonID: dead.ID, Type: "Immigration", Date: "1905", Place: "Durban"})
	s.CreateEvent(&store.Event{PersonID: dead.ID, Type: "Military Service", Date: "1914", DateEnd: "1918"})
	s.CreateEvent(&store.Event{PersonID: dead.ID, Type: "Undated", Date: ""})
	s.CreateEvent(&store.Event{PersonID: alive.ID, Type: "Occupation", Date: "2015", Place: "Cape Town"})
	people := []store.Person{*dead, *alive}

	opts := HTMLExportOptions{}
	page := generateTimelinePage(s, people, opts)
	for _, want := range []string{"Immigration", "in Durban", "Military Service", "1914 - 1918", "Occupation", "2015"} {
		if !strings.Contains(page, want) {
			t.Errorf("timeline page missing %q", want)
		}
	}
	if strings.Contains(page, "Undated") {
		t.Error("undated life event should not appear on the timeline")
	}

	person := generatePersonPage(s, dead, opts)
	if !strings.Contains(person, "<h2>Life Events</h2>") || !strings.Contains(person, "Immigration") ||
		strings.Count(person, "Life Events") != 1 {
		t.Errorf("person page life events section wrong:\n%s", person)
	}

	// Limited-info privacy hides a living person's events everywhere
	limited := HTMLExportOptions{LimitLivingInfo: true}
	if page := generateTimelinePage(s, people, limited); strings.Contains(page, "Occupation") || strings.Contains(page, "Cape Town") {
		t.Error("living person's life event leaked on timeline with LimitLivingInfo")
	}
	if page := generatePersonPage(s, alive, limited); strings.Contains(page, "Occupation") || strings.Contains(page, "Life Events") {
		t.Error("living person's life event leaked on person page with LimitLivingInfo")
	}
	// A person with no events gets no empty section
	bare := &store.Person{GivenName: "Bo", Surname: "Berg"}
	s.CreatePerson(bare)
	if page := generatePersonPage(s, bare, opts); strings.Contains(page, "Life Events") {
		t.Error("empty Life Events section rendered")
	}
}

func TestStreetViewURL(t *testing.T) {
	u := streetViewURL(-29.858681, 31.021840)
	if u == nil || u.Host != "www.google.com" || u.Path != "/maps/@" {
		t.Fatalf("url = %v", u)
	}
	q := u.Query()
	if q.Get("api") != "1" || q.Get("map_action") != "pano" || q.Get("viewpoint") != "-29.858681,31.021840" {
		t.Errorf("query = %v", q)
	}
}
