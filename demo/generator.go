// Package demo provides demo database generation functionality
package demo

import (
	"log"

	"genealogy/store"
)

// GenerateDemoData creates a comprehensive demo database with synthetic genealogical data
// Returns the ID of Michael Harrison (the suggested focus person)
func GenerateDemoData(s *store.Store) (int64, error) {
	// GENERATION 1: Great-great-grandparents (1880s-1890s)
	thomas := createPerson(s, "Thomas", "Harrison", "M", "15 Mar 1882", "Boston, Massachusetts, USA", "22 Jan 1959", "Boston, Massachusetts, USA", false, "")
	elizabeth := createPersonWithPreferred(s, "Elizabeth", "Morrison", "Beth", "F", "08 Jul 1885", "New York, New York, USA", "14 Nov 1963", "Boston, Massachusetts, USA", false, "")
	createMarriage(s, thomas, elizabeth, "12 Jun 1904", "Boston, Massachusetts, USA", "", "", "")

	william := createPersonWithPreferred(s, "William", "Foster", "Bill", "M", "23 Nov 1880", "Philadelphia, Pennsylvania, USA", "05 Mar 1957", "Chicago, Illinois, USA", false, "")
	margaret := createPerson(s, "Margaret", "Sullivan", "F", "30 Jan 1883", "Chicago, Illinois, USA", "18 Aug 1961", "Chicago, Illinois, USA", false, "")
	createMarriage(s, william, margaret, "14 Apr 1902", "Chicago, Illinois, USA", "", "", "")

	// GENERATION 2: Great-grandparents (1900s-1920s)
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
	michael := createPersonWithPreferred(s, "Michael John", "Harrison", "Mike", "M", "03 Mar 1955", "Austin, Texas, USA", "", "", true, "")
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

	christopher := createPersonWithPreferred(s, "Christopher", "Chen", "Chris", "M", "12 Oct 1960", "Denver, Colorado, USA", "", "", true, "")
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

	// Michael's children from first marriage
	brandon := createPerson(s, "Brandon", "Harrison", "M", "15 May 1978", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, michael, brandon)
	createParentChild(s, linda, brandon)

	ashley := createPerson(s, "Ashley", "Harrison", "F", "22 Aug 1980", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, michael, ashley)
	createParentChild(s, linda, ashley)

	// Michael's children from second marriage
	esther := createPerson(s, "Esther Ruth", "Harrison", "F", "07 Nov 1988", "Austin, Texas, USA", "", "", true, "123 Oak Street, Austin, TX 78701, USA")
	createParentChild(s, michael, esther)
	createParentChild(s, jennifer, esther)
	// Esther has full contact info
	esther.City = "Austin"
	esther.State = "Texas"
	esther.PostalCode = "78701"
	esther.Country = "USA"
	esther.Email = "esther.harrison.demo@example.com"
	esther.Phone = "+1 (512) 555-0198"
	s.UpdatePerson(esther)

	noah := createPerson(s, "Noah Daniel", "Harrison", "M", "14 Feb 1992", "Austin, Texas, USA", "", "", true, "")
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

	// Easter egg: Stanley Yelnats family from "Holes" - tracing back to Latvia
	// Generation 1: Great-great-grandfather from Latvia
	elya := createPerson(s, "Elya", "Yelnats", "M", "12 Aug 1850", "Riga, Latvia", "03 Nov 1920", "New York, New York, USA", false, "")
	elya.Notes = "Immigrated from Latvia to America in 1870s, first settling in Boston before later moving the family to New York City. Founded family tradition of naming first sons 'Stanley Yelnats'."
	s.UpdatePerson(elya)

	// Married in Boston (deliberately distinct from his birth and death
	// places) so his single lifetime shows 3 map locations - useful for
	// demoing the Map View's timeline slider: Riga -> Boston -> New York.
	sarah_yelnats := createPerson(s, "Sarah", "Miller", "F", "22 Apr 1855", "New York, New York, USA", "18 Dec 1925", "New York, New York, USA", false, "")
	createMarriage(s, elya, sarah_yelnats, "14 Jun 1875", "Boston, Massachusetts, USA", "", "", "")

	// Generation 2: Great-grandfather Stanley Yelnats I
	stanley1 := createPerson(s, "Stanley I", "Yelnats", "M", "05 Mar 1880", "New York, New York, USA", "22 Sep 1950", "Dallas, Texas, USA", false, "")
	createParentChild(s, elya, stanley1)
	createParentChild(s, sarah_yelnats, stanley1)

	mary_yelnats := createPerson(s, "Mary", "O'Brien", "F", "15 Jul 1882", "Boston, Massachusetts, USA", "11 Apr 1955", "Dallas, Texas, USA", false, "")
	createMarriage(s, stanley1, mary_yelnats, "10 Sep 1905", "New York, New York, USA", "", "", "")

	// Generation 3: Grandfather Stanley Yelnats II
	stanley2 := createPerson(s, "Stanley II", "Yelnats", "M", "28 Nov 1910", "New York, New York, USA", "16 Aug 1985", "Austin, Texas, USA", false, "")
	createParentChild(s, stanley1, stanley2)
	createParentChild(s, mary_yelnats, stanley2)

	thelma := createPerson(s, "Thelma", "Gladstone", "F", "03 Jan 1915", "Dallas, Texas, USA", "29 Dec 1995", "Austin, Texas, USA", false, "")
	createMarriage(s, stanley2, thelma, "22 Jun 1935", "Dallas, Texas, USA", "", "", "")

	// Generation 4: Father Stanley Yelnats III
	stanley3 := createPerson(s, "Stanley III", "Yelnats", "M", "17 Apr 1955", "Dallas, Texas, USA", "", "", true, "")
	createParentChild(s, stanley2, stanley3)
	createParentChild(s, thelma, stanley3)

	rebecca := createPersonWithPreferred(s, "Rebecca", "Stone", "Becky", "F", "08 Sep 1958", "Houston, Texas, USA", "", "", true, "")
	createMarriage(s, stanley3, rebecca, "15 May 1980", "Austin, Texas, USA", "", "", "")

	// Generation 5: Current Stanley Yelnats IV (married to Esther Harrison)
	stanley4 := createPerson(s, "Stanley IV", "Yelnats", "M", "10 Jul 1985", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, stanley3, stanley4)
	createParentChild(s, rebecca, stanley4)
	stanley4.Notes = "Fourth generation Stanley Yelnats. Continuing the family tradition of the palindrome name."
	s.UpdatePerson(stanley4)

	// Stanley IV marries Esther Ruth Harrison (creating connection to main Harrison family)
	createMarriage(s, stanley4, esther, "25 Jun 2015", "Austin, Texas, USA", "", "", "")

	mia := createPerson(s, "Mia", "Johnson", "F", "30 Oct 1991", "Boulder, Colorado, USA", "", "", true, "")
	createMarriage(s, ryan, mia, "14 Sep 2018", "Denver, Colorado, USA", "", "", "")

	// Recent children (Gen 6 preview)
	liam := createPerson(s, "Liam", "Yelnats", "M", "18 Mar 2018", "Austin, Texas, USA", "", "", true, "")
	createParentChild(s, stanley4, liam)
	createParentChild(s, esther, liam)
	liam.Notes = "First child not named Stanley Yelnats in five generations!"
	s.UpdatePerson(liam)

	// Intentional conflict: Liam born 2018, but parent Esther born 1988 (age 30 at birth - acceptable)
	// But let's add an impossible one:
	emily := createPerson(s, "Emily", "Harrison", "F", "01 Jan 2005", "Unknown", "", "", true, "")
	createParentChild(s, noah, emily) // Noah born 1992, Emily born 2005 (Noah would be 13 - should trigger Conflicts Report)

	// Add a few incomplete records
	_ = createPerson(s, "Unknown", "Harrison", "M", "", "", "", "", false, "")
	_ = createPerson(s, "Jane", "", "F", "", "", "", "", false, "")

	// Add notes to a few people
	esther.Notes = "Family genealogist. Maintains this database. Contact for family reunion information."
	s.UpdatePerson(esther)

	thomas.Notes = "Son of English immigrants who arrived in Boston in 1880. Founded Harrison & Co. shipping business in 1905."
	s.UpdatePerson(thomas)

	patricia.Notes = "Teacher at San Antonio Elementary School for 35 years. Beloved by generations of students."
	s.UpdatePerson(patricia)

	// Return Michael Harrison's ID as the suggested focus person (he has two marriages, showcasing that feature)
	return michael.ID, nil
}

func createPerson(s *store.Store, given, surname, gender, birthDate, birthPlace, deathDate, deathPlace string, isLiving bool, address string) *store.Person {
	return createPersonWithPreferred(s, given, surname, "", gender, birthDate, birthPlace, deathDate, deathPlace, isLiving, address)
}

func createPersonWithPreferred(s *store.Store, given, surname, preferredName, gender, birthDate, birthPlace, deathDate, deathPlace string, isLiving bool, address string) *store.Person {
	p := &store.Person{
		GivenName:     given,
		Surname:       surname,
		PreferredName: preferredName,
		Gender:        gender,
		BirthDate:     birthDate,
		BirthPlace:    birthPlace,
		DeathDate:     deathDate,
		DeathPlace:    deathPlace,
		IsLiving:      isLiving,
		Address:       address,
	}
	if err := s.CreatePerson(p); err != nil {
		log.Printf("Warning: Failed to create person %s %s: %v", given, surname, err)
	}
	return p
}

func createParentChild(s *store.Store, parent, child *store.Person) {
	rel := &store.Relationship{
		SubjectID: parent.ID,
		ObjectID:  child.ID,
		Type:      "child",
	}
	if err := s.CreateRelationship(rel); err != nil {
		log.Printf("Warning: Failed to create parent-child relationship: %v", err)
	}
}

func createMarriage(s *store.Store, person1, person2 *store.Person, marriageDate, marriagePlace, endDate, endPlace, endReason string) {
	// Create spouse relationship
	rel := &store.Relationship{
		SubjectID:     person1.ID,
		ObjectID:      person2.ID,
		Type:          "spouse",
		MarriageDate:  marriageDate,
		MarriagePlace: marriagePlace,
		DivorceDate:   endDate,
		EndReason:     endReason,
	}

	if err := s.CreateRelationship(rel); err != nil {
		log.Printf("Warning: Failed to create marriage: %v", err)
	}
}

// GetDemoStats returns statistics about the generated demo data
func GetDemoStats() string {
	return `The Harrison Family Demo Database contains:

📊 Demographics:
  • 50+ people across 5-6 generations (1850s-2020s)
  • Geographic diversity: USA (TX, CO, WA, CA, GA, AZ, OR, MA, IL, PA, NY) + Latvia
  • Living people with modern contact information
  • 🎭 Easter egg: Stanley Yelnats family line from "Holes"!

👤 Focus Person: Michael Harrison
  • Set as the default focus person to showcase this feature
  • Has 2 marriages (divorced first, remarried)
  • Press 'G' to return to focus person at any time

📷 Media Included:
  • Photos and documents for Michael, Jennifer, Esther & Noah
  • Look for 📷 icons next to names in all views
  • Click "View Media" button on people with media

🎯 Features Demonstrated:
  ✓ Multiple marriages & divorces (Michael Harrison)
  ✓ Living people with contact info (Esther, Mary)
  ✓ Preferred names/nicknames (Mike, Bill, Beth, Chris)
  ✓ Duplicate names (Thomas Harrison & 4x Stanley Yelnats!)
  ✓ International immigration (Elya Yelnats from Latvia)
  ✓ Multi-generational naming tradition (Stanley I-IV)
  ✓ Impossible relationships (Noah → Emily, parent too young)
  ✓ Incomplete records (Unknown, Jane with no surname)
  ✓ Living status issues (Mary, age 89, marked as living)
  ✓ Biographical notes on key individuals
  ✓ Media attachments (photos, documents)

💡 Try These Features:
  • Browse all 3 views (Family, Pedigree, Individual)
  • Run Reports → Data Quality Report
  • Check Reports → Conflicts Report
  • Use Reports → Statistics Dashboard
  • Search for "Esther" (has contact info & a surprise husband!)
  • Look for the palindrome family (hint: check Esther's spouse)
  • Try the relationship calculator
  • Use Media Library to browse all media`
}
