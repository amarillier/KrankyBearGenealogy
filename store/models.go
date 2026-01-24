package store

import "time"

// Person represents an individual in the genealogy DB.
type Person struct {
	ID           int64  `json:"id"`
	GivenName    string `json:"given_name"`
	Surname      string `json:"surname"`
	PreferredName string `json:"preferred_name"` // Name they prefer to be called (e.g., "Allan" instead of "Ivan")
	Gender       string `json:"gender"`

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
	
	// Research tracking
	Bookmarked   bool       `json:"bookmarked"`    // true if person is bookmarked/starred
	LastAccessed *time.Time `json:"last_accessed"` // timestamp of last view
	
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
	ID           int64     `json:"id"`
	Title        string    `json:"title"`        // e.g., "1900 US Federal Census"
	Author       string    `json:"author"`       // Author or creator
	Publication  string    `json:"publication"`  // Publication info (book, website, archive)
	Repository   string    `json:"repository"`   // Where it's held (archive, library, website)
	CallNumber   string    `json:"call_number"`  // Reference number, film number, URL
	Notes        string    `json:"notes"`        // Additional notes about the source
	SourceType   string    `json:"source_type"`  // "vital_record", "census", "church", "book", "website", "other"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Citation links a source to a person with specific details.
type Citation struct {
	ID             int64     `json:"id"`
	SourceID       int64     `json:"source_id"`
	PersonID       int64     `json:"person_id"`
	CitationDetail string    `json:"citation_detail"` // Specific page, entry, line number
	Transcription  string    `json:"transcription"`   // Transcription of relevant text
	Confidence     string    `json:"confidence"`      // "high", "medium", "low" - how reliable is this source
	Notes          string    `json:"notes"`           // Notes specific to this citation
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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

// ValidatedItem represents a conflict or duplicate that has been reviewed and validated.
type ValidatedItem struct {
	ID              int64     `json:"id"`
	ItemType        string    `json:"item_type"`         // "conflict", "duplicate"
	PersonID        int64     `json:"person_id"`
	RelatedPersonID *int64    `json:"related_person_id"` // For duplicates or relationship conflicts
	ConflictType    string    `json:"conflict_type"`     // "Parent Too Young", "Duplicate", etc.
	ValidationNote  string    `json:"validation_note"`
	ReviewedBy      string    `json:"reviewed_by"`
	ValidatedAt     time.Time `json:"validated_at"`
}

// ResearchTodo represents a research task/todo item for a person.
type ResearchTodo struct {
	ID          int64      `json:"id"`
	PersonID    int64      `json:"person_id"`
	Description string     `json:"description"`     // The task description
	Priority    string     `json:"priority"`        // "low", "medium", "high"
	Status      string     `json:"status"`          // "pending", "completed"
	Notes       string     `json:"notes"`           // Additional details/notes
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`    // When marked complete
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ResearchLog represents a research activity log entry - tracks what sources/records have been searched.
type ResearchLog struct {
	ID           int64      `json:"id"`
	PersonID     *int64     `json:"person_id"`      // Optional: link to specific person being researched
	SearchDate   time.Time  `json:"search_date"`    // When the search was conducted
	Repository   string     `json:"repository"`     // Where you searched (archive, website, library, FamilySearch, etc.)
	RecordType   string     `json:"record_type"`    // Type of record searched (census, vital records, newspapers, etc.)
	SearchGoal   string     `json:"search_goal"`    // What you were looking for
	Results      string     `json:"results"`        // What you found (or "Nothing found")
	Notes        string     `json:"notes"`          // Additional notes, URLs, reference numbers
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
