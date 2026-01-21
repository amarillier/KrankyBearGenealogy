// generate_demo.go - Creates a demo database with synthetic genealogical data
// Run with: go run generate_demo.go
// Output: ./sampledata/demo.db

// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/amarillier/KrankyBearGenealogy/store"
)

func main() {
	// Ensure sampledata directory exists
	if err := os.MkdirAll("./sampledata", 0755); err != nil {
		log.Fatalf("Failed to create sampledata directory: %v", err)
	}

	// Remove old demo.db if it exists
	demoPath := "./sampledata/demo.db"
	os.Remove(demoPath)

	// Create new database
	s, err := store.Open(demoPath)
	if err != nil {
		log.Fatalf("Failed to create demo database: %v", err)
	}
	defer s.Close()

	// Initialize schema
	if err := s.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	fmt.Println("Creating demo database with synthetic genealogical data...")
	fmt.Println("This showcases KrankyBear Genealogy features:")
	fmt.Println("  • 5 generations (~60 people)")
	fmt.Println("  • Multiple marriages & divorces")
	fmt.Println("  • Living people with contact info")
	fmt.Println("  • Geographic diversity")
	fmt.Println("  • Some data quality issues (for demo reports)")
	fmt.Println()

	// Generate the family tree
	if err := generateDemoFamily(s); err != nil {
		log.Fatalf("Failed to generate demo data: %v", err)
	}

	fmt.Println("\n✅ Demo database created successfully: ./sampledata/demo.db")
	fmt.Println("   Load it in the app via: Help → Load Demo Database")
}

func generateDemoFamily(s *store.Store) error {
	// GENERATION 1: Great-great-grandparents (1880s-1890s)
	fmt.Println("Creating Generation 1 (Great-great-grandparents)...")

	thomas := createPerson(s, "Thomas", "Harrison", "M", "15 Mar 1882", "Boston, Massachusetts, USA", "22 Jan 1959", "Boston, Massachusetts, USA", false, "")
	elizabeth := createPerson(s, "Elizabeth", "Morrison", "F", "08 Jul 1885", "New York, New York, USA", "14 Nov 1963", "Boston, Massachusetts, USA", false, "")
	createMarriage(s, thomas, elizabeth, "12 Jun 1904", "Boston, Massachusetts, USA", "", "", "")

	william := createPerson(s, "William", "Foster", "M", "23 Nov 1880", "Philadelphia, Pennsylvania, USA", "05 Mar 1957", "Chicago, Illinois, USA", false, "")
	margaret := createPerson(s, "Margaret", "Sullivan", "F", "30 Jan 1883", "Chicago, Illinois, USA", "18 Aug 1961", "Chicago, Illinois, USA", false, "")
	createMarriage(s, william, margaret, "14 Apr 1902", "Chicago, Illinois, USA", "", "", "")

	// GENERATION 2: Great-grandparents (1900s-1920s)
	fmt.Println("Creating Generation 2 (Great-grandparents)...")

	robert := createPerson(s, "Robert", "Harrison", "M", "28 Sep 1905", "Boston, Massachusetts, USA", "11 Dec 1982", "Denver, Colorado, USA", false, "")
	createParentChild(s, thomas, robert)
	createParentChild(s, elizabeth, robert)

	dorothy := createPerson(s, "Dorothy", "Foster", "F", "14 Feb 1907", "Chicago, Illinois, USA", "29 May 1989", "Denver, Colorado, USA", false, "")
	createParentChild(s, william, dorothy)
	createParentChild(s, margaret, dorothy)
	createMarriage(s, robert, dorothy, "22 Jun 1925", "Chicago, Illinois, USA", "", "", "")

	charles := createPerson(s, "Charles", "Harrison", "M", "03 Apr 1910", "Boston, Massachusetts, USA", "16 Jul 1975", "Boston, Massachusetts, USA", false, "")
	createParentChild(s, thomas, charles)
	createParentChild(s, elizabeth, charles)

	helen := createPerson(s, "Helen", "Campbell", "F", "19 Oct 1912", "Portland, Oregon, USA", "08 Sep 1997", "Seattle, Washington, USA", false, "")
	createMarriage(s, charles, helen, "05 May 1933", "Portland, Oregon, USA", "", "", "")

	// GENERATION 3: Grandparents (1930s-1950s)
	fmt.Println("Creating Generation 3 (Grandparents)...")

	james := createPerson(s, "James", "Harrison", "M", "17 Jan 1930", "Denver, Colorado, USA", "23 Apr 2015", "Austin, Texas, USA", false, "")
	createParentChild(s, robert, james)
	createParentChild(s, dorothy, james)

	patricia := createPerson(s, "Patricia", "Rodriguez", "F", "25 Aug 1932", "San Antonio, Texas, USA", "14 Nov 2018", "Austin, Texas, USA", false, "")
	createMarriage(s, james, patricia, "12 Sep 1952", "San Antonio, Texas, USA", "", "", "")

	mary := createPerson(s, "Mary", "Harrison", "F", "08 Nov 1935", "Denver, Colorado, USA", "", "", true, "456 Mountain View Lane, Denver, CO 80202, USA")
	createParentChild(s, robert, mary)
	createParentChild(s, dorothy, mary)
	// Mary has contact info (demonstrating living person tracking)
	mary.City = "Denver"
	mary.State = "Colorado"
	mary.PostalCode = "80202"
	mary.Country = "USA"
	mary.Email = "mary.harrison.demo@example.com"
	mary.Phone = "+1 (303) 555-0142"
	s.UpdatePerson(mary)

	// Intentional data quality issue: Mary is 89 years old but marked as living (should trigger Living Status Report)

	david := createPerson(s, "David", "Chen", "M", "22 May 1933", "San Francisco, California, USA", "", "", true, "")
	createMarriage(s, david, mary, "18 Jul 1955", "Denver, Colorado, USA", "", "", "")

	john := createPerson(s, "John", "Harrison", "M", "30 Dec 1938", "Seattle, Washington, USA", "07 Feb 2001", "Seattle, Washington, USA", false, "")
	createParentChild(s, charles, john)
	createParentChild(s, helen, john)

	susan := createPerson(s, "Susan", "Anderson", "F", "11 Apr 1940", "Portland, Oregon, USA", "", "", true, "")
	createMarriage(s, john, susan, "15 Jun 1960", "Seattle, Washington, USA", "", "", "")

	// GENERATION 4: Parents (1950s-1980s) - Include divorces and remarriages
	fmt.Println("Creating Generation 4 (Parents - with multiple marriages)...")

	michael := createPerson(s, "Michael", "Harrison", "M", "03 Mar 1955", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, james, michael)
	createParentChild(s, patricia, michael)

	// Michael's first marriage (ended in divorce)
	linda := createPerson(s, "Linda", "Taylor", "F", "18 Sep 1956", "Houston, Texas, USA", "", "", true, "")
	createMarriage(s, michael, linda, "22 Aug 1975", "Austin, Texas, USA", "1982", "", "Divorce")

	// Michael's second marriage (current)
	jennifer := createPerson(s, "Jennifer", "Martinez", "F", "27 Nov 1960", "Dallas, Texas, USA", "", "", true, "")
	createMarriage(s, michael, jennifer, "14 Feb 1985", "Austin, Texas, USA", "", "", "")

	sarah := createPerson(s, "Sarah", "Harrison", "F", "19 Jul 1958", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, james, sarah)
	createParentChild(s, patricia, sarah)

	daniel := createPerson(s, "Daniel", "Brooks", "M", "05 Jan 1957", "Phoenix, Arizona, USA", "", "", true, "")
	createMarriage(s, daniel, sarah, "30 Jun 1980", "Austin, Texas, USA", "", "", "")

	christopher := createPerson(s, "Christopher", "Chen", "M", "12 Oct 1960", "Denver, Colorado, USA", "", "", true, "")
	createParentChild(s, david, christopher)
	createParentChild(s, mary, christopher)

	angela := createPerson(s, "Angela", "Washington", "F", "08 Apr 1962", "Atlanta, Georgia, USA", "", "", true, "")
	createMarriage(s, christopher, angela, "25 May 1985", "Denver, Colorado, USA", "", "", "")

	thomas2 := createPerson(s, "Thomas", "Harrison", "M", "16 Feb 1965", "Seattle, Washington, USA", "", "", true, "")
	createParentChild(s, john, thomas2)
	createParentChild(s, susan, thomas2)

	// Intentional duplicate name (same as great-great-grandfather) to demo Duplicate Detection
	// This Thomas is marked with birthdate 1965 and living

	rachel := createPerson(s, "Rachel", "Kim", "F", "29 Nov 1967", "San Jose, California, USA", "", "", true, "")
	createMarriage(s, thomas2, rachel, "10 Sep 1990", "Seattle, Washington, USA", "", "", "")

	// GENERATION 5: Current generation (1980s-2020s)
	fmt.Println("Creating Generation 5 (Current generation)...")

	// Michael's children from first marriage
	brandon := createPerson(s, "Brandon", "Harrison", "M", "15 May 1978", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, michael, brandon)
	createParentChild(s, linda, brandon)

	ashley := createPerson(s, "Ashley", "Harrison", "F", "22 Aug 1980", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, michael, ashley)
	createParentChild(s, linda, ashley)

	// Michael's children from second marriage
	emma := createPerson(s, "Emma", "Harrison", "F", "07 Nov 1988", "Austin, Texas, USA", "", "", true, "123 Oak Street, Austin, TX 78701, USA")
	createParentChild(s, michael, emma)
	createParentChild(s, jennifer, emma)
	// Emma has full contact info
	emma.City = "Austin"
	emma.State = "Texas"
	emma.PostalCode = "78701"
	emma.Country = "USA"
	emma.Email = "emma.harrison.demo@example.com"
	emma.Phone = "+1 (512) 555-0198"
	s.UpdatePerson(emma)

	noah := createPerson(s, "Noah", "Harrison", "M", "14 Feb 1992", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, michael, noah)
	createParentChild(s, jennifer, noah)

	// Sarah's children
	olivia := createPerson(s, "Olivia", "Brooks", "F", "03 Jun 1985", "Phoenix, Arizona, USA", "", "", true, "")
	createParentChild(s, daniel, olivia)
	createParentChild(s, sarah, olivia)

	ethan := createPerson(s, "Ethan", "Brooks", "M", "28 Sep 1988", "Phoenix, Arizona, USA", "", "", true, "")
	createParentChild(s, daniel, ethan)
	createParentChild(s, sarah, ethan)

	sophia := createPerson(s, "Sophia", "Brooks", "F", "11 Jan 1993", "Phoenix, Arizona, USA", "", "", true, "")
	createParentChild(s, daniel, sophia)
	createParentChild(s, sarah, sophia)

	// Christopher's children
	ryan := createPerson(s, "Ryan", "Chen", "M", "19 Mar 1990", "Denver, Colorado, USA", "", "", true, "")
	createParentChild(s, christopher, ryan)
	createParentChild(s, angela, ryan)

	madison := createPerson(s, "Madison", "Chen", "F", "25 Jul 1993", "Denver, Colorado, USA", "", "", true, "")
	createParentChild(s, christopher, madison)
	createParentChild(s, angela, madison)

	// Thomas2's children
	jacob := createPerson(s, "Jacob", "Harrison", "M", "08 Dec 1995", "Seattle, Washington, USA", "", "", true, "")
	createParentChild(s, thomas2, jacob)
	createParentChild(s, rachel, jacob)

	// Intentional data quality issue: Jacob has no birth location (should trigger Data Quality Report)
	jacob.BirthPlace = ""
	s.UpdatePerson(jacob)

	ava := createPerson(s, "Ava", "Harrison", "F", "16 Apr 2000", "Seattle, Washington, USA", "", "", true, "")
	createParentChild(s, thomas2, ava)
	createParentChild(s, rachel, ava)

	// Add some spouses for Gen 5 (showing in-law relationships)
	alex := createPerson(s, "Alex", "Thompson", "M", "12 Feb 1987", "Portland, Oregon, USA", "", "", true, "")
	createMarriage(s, alex, emma, "25 Jun 2015", "Austin, Texas, USA", "", "", "")

	mia := createPerson(s, "Mia", "Johnson", "F", "30 Oct 1991", "Boulder, Colorado, USA", "", "", true, "")
	createMarriage(s, ryan, mia, "14 Sep 2018", "Denver, Colorado, USA", "", "", "")

	// Recent children (Gen 6 preview)
	liam := createPerson(s, "Liam", "Thompson", "M", "18 Mar 2018", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, alex, liam)
	createParentChild(s, emma, liam)

	// Intentional conflict: Liam born 2018, but parent Emma born 1988 (age 30 at birth - acceptable)
	// But let's add an impossible one:
	emily := createPerson(s, "Emily", "Harrison", "F", "01 Jan 2005", "Unknown", "", "", true, "")
	createParentChild(s, noah, emily) // Noah born 1992, Emily born 2005 (Noah would be 13 - should trigger Conflicts Report)

	// Add a few incomplete records
	fmt.Println("Adding incomplete records (for Data Quality Report)...")
	_ = createPerson(s, "Unknown", "Harrison", "M", "", "", "", "", false, "")
	_ = createPerson(s, "Jane", "", "F", "", "", "", "", false, "")

	// Add notes to a few people
	emma.Notes = "Family genealogist. Maintains this database. Contact for family reunion information."
	s.UpdatePerson(emma)

	thomas.Notes = "Immigrated to Boston from England in 1880. Founded Harrison & Co. shipping business."
	s.UpdatePerson(thomas)

	patricia.Notes = "Teacher at San Antonio Elementary School for 35 years. Beloved by generations of students."
	s.UpdatePerson(patricia)

	fmt.Println("\nGenerated:")
	count, _ := s.CountPeople()
	fmt.Printf("  • %d people across 5 generations\n", count)
	relCount, _ := s.GetRelationshipCount()
	fmt.Printf("  • %d relationships\n", relCount)

	fmt.Println("\nDemo features:")
	fmt.Println("  ✓ Multiple marriages (Michael Harrison)")
	fmt.Println("  ✓ Living people with contact info (Emma, Mary)")
	fmt.Println("  ✓ Geographic diversity (TX, CO, WA, CA, GA, etc.)")
	fmt.Println("  ✓ Duplicate names (Thomas Harrison appears twice)")
	fmt.Println("  ✓ Impossible relationship (Noah → Emily, parent too young)")
	fmt.Println("  ✓ Incomplete records (Unknown, Jane with no surname)")
	fmt.Println("  ✓ Living status issues (Mary, age 89, marked as living)")
	fmt.Println("  ✓ Notes on key individuals")

	return nil
}

func createPerson(s *store.Store, given, surname, gender, birthDate, birthPlace, deathDate, deathPlace string, isLiving bool, address string) *store.Person {
	p := &store.Person{
		GivenName:  given,
		Surname:    surname,
		Gender:     gender,
		BirthDate:  birthDate,
		BirthPlace: birthPlace,
		DeathDate:  deathDate,
		DeathPlace: deathPlace,
		IsLiving:   isLiving,
		Address:    address,
	}
	if err := s.CreatePerson(p); err != nil {
		log.Printf("Warning: Failed to create person %s %s: %v", given, surname, err)
	}
	return p
}

func createParentChild(s *store.Store, parent, child *store.Person) {
	if err := s.CreateRelationship(parent.ID, child.ID, "parent"); err != nil {
		log.Printf("Warning: Failed to create parent-child relationship: %v", err)
	}
}

func createMarriage(s *store.Store, person1, person2 *store.Person, marriageDate, marriagePlace, endDate, endPlace, endReason string) {
	marriage := &store.Marriage{
		MarriageDate:  marriageDate,
		MarriagePlace: marriagePlace,
		EndDate:       endDate,
		EndPlace:      endPlace,
		EndReason:     endReason,
	}
	
	if err := s.CreateSpouseRelationship(person1.ID, person2.ID, marriage); err != nil {
		log.Printf("Warning: Failed to create marriage: %v", err)
	}
}
