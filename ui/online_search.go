package ui

import (
	"fmt"
	"net/url"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"genealogy/store"
)

// ancestrySearchURL builds an Ancestry.com name search URL for a person.
// Best-effort convenience link - Ancestry's search query parameters are
// third-party and may change over time.
func ancestrySearchURL(p *store.Person) *url.URL {
	values := url.Values{}
	values.Set("name", fmt.Sprintf("%s_%s", p.GivenName, p.Surname))
	if year := extractYearFromDate(p.BirthDate); year > 0 {
		values.Set("birth", strconv.Itoa(year))
	}
	u, _ := url.Parse("https://www.ancestry.com/search/?" + values.Encode())
	return u
}

// familySearchURL builds a FamilySearch.org record search URL for a person.
// Best-effort convenience link - FamilySearch's search query parameters are
// third-party and may change over time.
func familySearchURL(p *store.Person) *url.URL {
	values := url.Values{}
	values.Set("q.givenName", p.GivenName)
	values.Set("q.surname", p.Surname)
	if year := extractYearFromDate(p.BirthDate); year > 0 {
		yearStr := strconv.Itoa(year)
		values.Set("q.birthLikeDate.from", yearStr)
		values.Set("q.birthLikeDate.to", yearStr)
	}
	u, _ := url.Parse("https://www.familysearch.org/search/record/results?" + values.Encode())
	return u
}

// genealogySearchQuery builds a general web-search query biased toward
// genealogy content: the name as an exact phrase, plus "genealogy" and the
// birth year when known.
func genealogySearchQuery(p *store.Person) string {
	query := fmt.Sprintf("%q genealogy", fmt.Sprintf("%s %s", p.GivenName, p.Surname))
	if year := extractYearFromDate(p.BirthDate); year > 0 {
		query += " " + strconv.Itoa(year)
	}
	return query
}

// googleSearchURL builds a Google search URL for a person.
func googleSearchURL(p *store.Person) *url.URL {
	values := url.Values{}
	values.Set("q", genealogySearchQuery(p))
	u, _ := url.Parse("https://www.google.com/search?" + values.Encode())
	return u
}

// bingSearchURL builds a Bing search URL for a person.
func bingSearchURL(p *store.Person) *url.URL {
	values := url.Values{}
	values.Set("q", genealogySearchQuery(p))
	u, _ := url.Parse("https://www.bing.com/search?" + values.Encode())
	return u
}

// duckDuckGoSearchURL builds a DuckDuckGo search URL for a person.
func duckDuckGoSearchURL(p *store.Person) *url.URL {
	values := url.Values{}
	values.Set("q", genealogySearchQuery(p))
	u, _ := url.Parse("https://duckduckgo.com/?" + values.Encode())
	return u
}

// openOnlineSearch opens the given URL in the user's default browser.
func openOnlineSearch(u *url.URL, w fyne.Window) {
	if u == nil {
		return
	}
	if err := fyne.CurrentApp().OpenURL(u); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to open browser: %w", err), w)
	}
}
