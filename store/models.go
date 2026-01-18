package store

import "time"

// Person represents an individual in the genealogy DB.
type Person struct {
	ID        int64  `json:"id"`
	GivenName string `json:"given_name"`
	Surname   string `json:"surname"`
	Gender    string `json:"gender"`

	BirthDate  string    `json:"birth_date"` // YYYY-MM-DD format
	BirthPlace string    `json:"birth_place"`
	DeathDate  string    `json:"death_date"` // YYYY-MM-DD format
	DeathPlace string    `json:"death_place"`
	IsLiving   bool      `json:"is_living"` // true if still living
	
	// Contact information (for living relatives - not part of GEDCOM standard)
	Address    string `json:"address"`     // Street address
	City       string `json:"city"`        // City
	State      string `json:"state"`       // State/Province
	PostalCode string `json:"postal_code"` // ZIP/Postal code
	Country    string `json:"country"`     // Country
	Email      string `json:"email"`       // Email address
	Phone      string `json:"phone"`       // Phone number
	
	UID        string    `json:"uid"`       // external / GEDCOM UID
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Family represents a family unit.
type Family struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Relationship describes a typed relationship between two persons.
type Relationship struct {
	ID              int64     `json:"id"`
	SubjectID       int64     `json:"subject_id"`       // source person
	ObjectID        int64     `json:"object_id"`        // target person
	Type            string    `json:"type"`             // e.g., "parent", "child", "spouse", "partner", "sibling", "other"
	MarriageDate    string    `json:"marriage_date"`    // for spouse/partner relationships
	MarriagePlace   string    `json:"marriage_place"`   // for spouse/partner relationships
	DivorceDate     string    `json:"divorce_date"`     // end date for spouse/partner relationships
	SeparationDate  string    `json:"separation_date"`  // separation date if different from divorce
	EndReason       string    `json:"end_reason"`       // "divorce", "death", "separation", etc.
	CreatedAt       time.Time `json:"created_at"`
}

// Event represents an event associated with a person (birth, death, marriage, burial, occupation, etc.).
type Event struct {
	ID        int64     `json:"id"`
	PersonID  int64     `json:"person_id"`
	Type      string    `json:"type"` // e.g., BIRT, DEAT, MARR, BURI, OCCU
	Date      string    `json:"date"`
	Place     string    `json:"place"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// Identifier stores external identifiers or GEDCOM UID/REFN entries for a person.
type Identifier struct {
	ID        int64     `json:"id"`
	PersonID  int64     `json:"person_id"`
	Tag       string    `json:"tag"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// Source represents a bibliographic/source citation.
type Source struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Citation  string    `json:"citation"`
	CreatedAt time.Time `json:"created_at"`
}

// Media represents a media file (photo, document, etc.) that can be linked to multiple people.
// Storage strategy: Primary = BLOBs in database for portability, Optional = external file path
type Media struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`          // User-provided title/caption
	Description  string    `json:"description"`    // Detailed description
	MediaType    string    `json:"media_type"`     // "image", "document", "audio", "video"
	MimeType     string    `json:"mime_type"`      // e.g., "image/jpeg", "image/png", "application/pdf"
	Thumbnail    []byte    `json:"-"`              // Small thumbnail (e.g., 150x150) for fast UI loading
	FullImage    []byte    `json:"-"`              // Full-resolution image stored as BLOB
	ExternalPath string    `json:"external_path"`  // Optional: path to external file (for power users)
	IsExternal   bool      `json:"is_external"`    // true if using external path, false if using BLOBs
	FileSize     int64     `json:"file_size"`      // Original file size in bytes
	DateTaken    string    `json:"date_taken"`     // Date photo was taken (YYYY-MM-DD format)
	LinkedPeople []int64   `json:"linked_people"`  // IDs of people linked to this media (populated on demand)
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
