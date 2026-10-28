// Package dedupe scores how likely two person records are the same individual.
// It is shared by the Duplicate Detection report and GEDCOM Merge.
package dedupe

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"genealogy/store"
)

// DefaultThreshold is the minimum overall score reported as a likely match.
const DefaultThreshold = 70.0

// Score breaks down a comparison of two people. All values are 0-100.
type Score struct {
	Name    float64
	Date    float64 // birth date similarity
	Overall float64
	// SameUID is true when both records carry the same non-empty UID.
	SameUID bool
}

// Compare scores a against b. Overall is 60% name and 40% birth date, then
// adjusted: a gender conflict or clearly different death years pull the score
// down, a matching birth place nudges it up, and a shared UID is certain.
func Compare(a, b store.Person) Score {
	s := Score{Name: NameScore(a, b), Date: DateScore(a.BirthDate, b.BirthDate)}
	if a.UID != "" && a.UID == b.UID {
		s.SameUID = true
		s.Overall = 100
		return s
	}

	overall := s.Name*0.6 + s.Date*0.4

	if ga, gb := normGender(a.Gender), normGender(b.Gender); ga != "" && gb != "" && ga != gb {
		overall *= 0.5
	}
	if da, db := ParseDate(a.DeathDate), ParseDate(b.DeathDate); da.Year != 0 && db.Year != 0 && abs(da.Year-db.Year) > 2 {
		overall *= 0.7
	}
	if pa, pb := normText(a.BirthPlace), normText(b.BirthPlace); pa != "" && pa == pb {
		overall += 5
	}
	if overall > 100 {
		overall = 100
	}
	s.Overall = overall
	return s
}

// Status classifies how an incoming person relates to existing records.
type Status int

const (
	StatusNew    Status = iota // no plausible match
	StatusLikely               // plausible match, needs a human decision
	StatusExact                // same UID, or identical name and matching birth date
)

func (s Status) String() string {
	switch s {
	case StatusExact:
		return "Exact match"
	case StatusLikely:
		return "Likely match"
	}
	return "New"
}

// Classify turns a score and the two records into a Status.
func Classify(sc Score, a, b store.Person, threshold float64) Status {
	if sc.SameUID {
		return StatusExact
	}
	if sc.Name >= 100 && sc.Date >= 90 && a.BirthDate != "" && b.BirthDate != "" &&
		(normGender(a.Gender) == normGender(b.Gender) || normGender(a.Gender) == "" || normGender(b.Gender) == "") &&
		sc.Overall >= 95 {
		return StatusExact
	}
	if sc.Overall > threshold {
		return StatusLikely
	}
	return StatusNew
}

// NameScore compares full names, 0-100. Identical names score 100; a name whose
// given part is a prefix of the other's (John / John William) with the same
// surname scores 90; otherwise it is Levenshtein similarity.
func NameScore(a, b store.Person) float64 {
	ga, sa := normText(a.GivenName), normText(a.Surname)
	gb, sb := normText(b.GivenName), normText(b.Surname)
	na := strings.TrimSpace(ga + " " + sa)
	nb := strings.TrimSpace(gb + " " + sb)
	if na == nb {
		if na == "" {
			return 0
		}
		return 100
	}
	best := similarity(na, nb)
	if sa != "" && sa == sb && ga != "" && gb != "" &&
		(strings.HasPrefix(ga, gb+" ") || strings.HasPrefix(gb, ga+" ")) && best < 90 {
		best = 90
	}
	return best
}

// DateScore compares two date strings, 0-100. Both blank is neutral (50), one
// blank is low (20). Full dates are bucketed by days apart; if either is only
// a year (or month), the comparison is by year.
func DateScore(da, db string) float64 {
	if strings.TrimSpace(da) == "" && strings.TrimSpace(db) == "" {
		return 50
	}
	if strings.TrimSpace(da) == "" || strings.TrimSpace(db) == "" {
		return 20
	}
	a, b := ParseDate(da), ParseDate(db)
	if a.Year == 0 || b.Year == 0 {
		return 20
	}
	if a.Day != 0 && b.Day != 0 && a.Month != 0 && b.Month != 0 {
		days := abs(a.days() - b.days())
		switch {
		case days < 1:
			return 100
		case days < 7:
			return 90
		case days < 30:
			return 80
		case days < 365:
			return 70
		case days < 365*2:
			return 50
		case days < 365*5:
			return 30
		}
		return 10
	}
	// Imprecise: a matching year cannot be confirmed to the day.
	switch d := abs(a.Year - b.Year); {
	case d == 0:
		return 85
	case d == 1:
		return 60
	case d == 2:
		return 40
	case d <= 5:
		return 25
	}
	return 10
}

// Date is the parsed pieces of a genealogy date; zero fields are unknown.
type Date struct{ Year, Month, Day int }

// days approximates a day count, good enough for bucketing differences.
func (d Date) days() int { return d.Year*365 + d.Month*30 + d.Day }

var (
	yearRe  = regexp.MustCompile(`\b(\d{4})\b`)
	isoRe   = regexp.MustCompile(`\b(\d{4})-(\d{1,2})-(\d{1,2})\b`)
	dmyRe   = regexp.MustCompile(`(?i)\b(\d{1,2})\s+([a-z]{3})[a-z]*\.?\s+(\d{4})\b`)
	myRe    = regexp.MustCompile(`(?i)\b([a-z]{3})[a-z]*\.?\s+(\d{4})\b`)
	monthNo = map[string]int{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
)

// ParseDate extracts year, month and day from GEDCOM or ISO style dates,
// ignoring qualifiers such as ABT, BEF and AFT. Unparseable text returns the
// zero Date.
func ParseDate(s string) Date {
	if m := isoRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return Date{y, mo, d}
	}
	if m := dmyRe.FindStringSubmatch(s); m != nil {
		if mo, ok := monthNo[strings.ToLower(m[2])]; ok {
			d, _ := strconv.Atoi(m[1])
			y, _ := strconv.Atoi(m[3])
			return Date{y, mo, d}
		}
	}
	if m := myRe.FindStringSubmatch(s); m != nil {
		if mo, ok := monthNo[strings.ToLower(m[1])]; ok {
			y, _ := strconv.Atoi(m[2])
			return Date{y, mo, 0}
		}
	}
	if m := yearRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		return Date{y, 0, 0}
	}
	return Date{}
}

// Pair is two people from the same set who look like the same individual.
type Pair struct {
	A, B  store.Person
	Score Score
}

// FindDuplicates returns pairs within people scoring above threshold, best
// first. Only people in the same surname-sound bucket are compared, so it
// scales far better than comparing every pair.
func FindDuplicates(people []store.Person, threshold float64) []Pair {
	idx := NewIndex(people)
	var pairs []Pair
	for i := range people {
		for _, j := range idx.candidateIndexes(people[i]) {
			if j <= i {
				continue
			}
			sc := Compare(people[i], people[j])
			if sc.Overall > threshold {
				pairs = append(pairs, Pair{people[i], people[j], sc})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].Score.Overall > pairs[j].Score.Overall })
	return pairs
}

// Candidate is an existing person that may match an incoming one.
type Candidate struct {
	Person store.Person
	Score  Score
	Status Status
}

// Index groups people by the sound of their surname so an incoming person is
// only compared against plausible matches.
type Index struct {
	people  []store.Person
	buckets map[string][]int
	byUID   map[string][]int
}

// NewIndex builds an Index over people.
func NewIndex(people []store.Person) *Index {
	idx := &Index{people: people, buckets: map[string][]int{}, byUID: map[string][]int{}}
	for i, p := range people {
		k := bucketKey(p)
		idx.buckets[k] = append(idx.buckets[k], i)
		if p.UID != "" {
			idx.byUID[p.UID] = append(idx.byUID[p.UID], i)
		}
	}
	return idx
}

func (idx *Index) candidateIndexes(p store.Person) []int {
	seen := map[int]bool{}
	var out []int
	add := func(list []int) {
		for _, i := range list {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	add(idx.buckets[bucketKey(p)])
	if p.UID != "" {
		add(idx.byUID[p.UID])
	}
	sort.Ints(out)
	return out
}

// Match returns existing people that plausibly are p, best first. People
// scoring at or below threshold are omitted.
func (idx *Index) Match(p store.Person, threshold float64) []Candidate {
	var out []Candidate
	for _, i := range idx.candidateIndexes(p) {
		sc := Compare(p, idx.people[i])
		if st := Classify(sc, p, idx.people[i], threshold); st != StatusNew {
			out = append(out, Candidate{idx.people[i], sc, st})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score.Overall > out[j].Score.Overall })
	return out
}

func bucketKey(p store.Person) string {
	if s := normText(p.Surname); s != "" {
		return "s:" + soundex(s)
	}
	return "g:" + soundex(normText(p.GivenName))
}

// soundex returns the classic 4-character Soundex code of a lowercase word.
func soundex(s string) string {
	codes := map[rune]byte{'b': '1', 'f': '1', 'p': '1', 'v': '1',
		'c': '2', 'g': '2', 'j': '2', 'k': '2', 'q': '2', 's': '2', 'x': '2', 'z': '2',
		'd': '3', 't': '3', 'l': '4', 'm': '5', 'n': '5', 'r': '6'}
	var out []byte
	var last byte
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		c := codes[r]
		if len(out) == 0 {
			out = append(out, byte(unicode.ToUpper(r)))
			last = c
			continue
		}
		if c != 0 && c != last {
			out = append(out, c)
		}
		if r != 'h' && r != 'w' {
			last = c
		}
		if len(out) == 4 {
			break
		}
	}
	for len(out) < 4 && len(out) > 0 {
		out = append(out, '0')
	}
	return string(out)
}

// normText lowercases and collapses punctuation/whitespace for comparison.
func normText(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		} else {
			space = true
		}
	}
	return b.String()
}

func normGender(g string) string {
	g = strings.ToLower(strings.TrimSpace(g))
	switch g {
	case "m", "male":
		return "m"
	case "f", "female":
		return "f"
	}
	return ""
}

func similarity(a, b string) float64 {
	maxLen := max(len([]rune(a)), len([]rune(b)))
	if maxLen == 0 {
		return 0
	}
	s := (1 - float64(levenshtein(a, b))/float64(maxLen)) * 100
	if s < 0 {
		return 0
	}
	return s
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
