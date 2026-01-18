package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// intFromBool converts a boolean to an integer (1 for true, 0 for false)
func intFromBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// boolFromInt converts an integer to a boolean (nonzero = true, 0 = false)
func boolFromInt(n int) bool {
	return n != 0
}

// Store wraps *sql.DB and provides simple helpers for the genealogy DB.
type Store struct {
	DB *sql.DB
}

// Open opens (and creates directories for) the sqlite DB at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Keep a single connection by default for simplicity.
	db.SetMaxOpenConns(1)

	// Enable foreign keys and WAL for better concurrency.
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		// non-fatal
	}

	return &Store{DB: db}, nil
}

// Close closes the underlying DB.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// InitSchema creates the basic tables needed for the app.
func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS persons (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            given_name TEXT,
            surname TEXT,
            gender TEXT,
            birth_year INTEGER,
            death_year INTEGER,
            birth_date TEXT,
            birth_place TEXT,
            death_date TEXT,
            death_place TEXT,
            is_living INTEGER DEFAULT 0,
            address TEXT,
            city TEXT,
            state TEXT,
            postal_code TEXT,
            country TEXT,
            email TEXT,
            phone TEXT,
            uid TEXT,
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS families (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS relationships (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            subject_id INTEGER NOT NULL,
            object_id INTEGER NOT NULL,
            type TEXT NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(subject_id) REFERENCES persons(id) ON DELETE CASCADE,
            FOREIGN KEY(object_id) REFERENCES persons(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            type TEXT NOT NULL,
            date TEXT,
            place TEXT,
            note TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS identifiers (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            tag TEXT,
            value TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS sources (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT,
            citation TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS media (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT,
            description TEXT,
            media_type TEXT DEFAULT 'image',
            mime_type TEXT,
            thumbnail BLOB,
            full_image BLOB,
            external_path TEXT,
            is_external INTEGER DEFAULT 0,
            file_size INTEGER DEFAULT 0,
            date_taken TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS person_media (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            media_id INTEGER NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
            FOREIGN KEY(media_id) REFERENCES media(id) ON DELETE CASCADE,
            UNIQUE(person_id, media_id)
        );`,
		`CREATE INDEX IF NOT EXISTS idx_person_name ON persons(surname, given_name);`,
		`CREATE INDEX IF NOT EXISTS idx_relationship_subject ON relationships(subject_id);`,
		`CREATE INDEX IF NOT EXISTS idx_relationship_object ON relationships(object_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_person ON events(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_person_media_person ON person_media(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_person_media_media ON person_media(media_id);`,
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// MigrateSchema applies schema migrations for new columns (safe to call multiple times).
func (s *Store) MigrateSchema() error {
	// These migrations add new columns if they don't already exist.
	// SQLite doesn't have IF NOT EXISTS for ALTER TABLE, so we check for errors and ignore duplicates.
	migrations := []string{
		`ALTER TABLE persons ADD COLUMN birth_date TEXT;`,
		`ALTER TABLE persons ADD COLUMN birth_place TEXT;`,
		`ALTER TABLE persons ADD COLUMN death_date TEXT;`,
		`ALTER TABLE persons ADD COLUMN death_place TEXT;`,
		`ALTER TABLE persons ADD COLUMN is_living INTEGER DEFAULT 0;`,
		`ALTER TABLE persons ADD COLUMN address TEXT;`,
		`ALTER TABLE persons ADD COLUMN city TEXT;`,
		`ALTER TABLE persons ADD COLUMN state TEXT;`,
		`ALTER TABLE persons ADD COLUMN postal_code TEXT;`,
		`ALTER TABLE persons ADD COLUMN country TEXT;`,
		`ALTER TABLE persons ADD COLUMN email TEXT;`,
		`ALTER TABLE persons ADD COLUMN phone TEXT;`,
		`ALTER TABLE relationships ADD COLUMN marriage_date TEXT;`,
		`ALTER TABLE relationships ADD COLUMN marriage_place TEXT;`,
		`ALTER TABLE relationships ADD COLUMN divorce_date TEXT;`,
		`ALTER TABLE relationships ADD COLUMN separation_date TEXT;`,
		`ALTER TABLE relationships ADD COLUMN end_reason TEXT;`,
	}
	for _, m := range migrations {
		if _, err := s.DB.Exec(m); err != nil {
			// Ignore "duplicate column" errors since the column may already exist
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}
	}
	
	// Add UNIQUE constraint on relationships to prevent duplicates
	// Use CREATE UNIQUE INDEX which is idempotent
	_, err := s.DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_relationship ON relationships(subject_id, object_id, type);`)
	if err != nil {
		return err
	}
	
	// Migrate media to many-to-many relationship
	// Check if media table has person_id column (old schema)
	var hasPersonID bool
	rows, err := s.DB.Query(`PRAGMA table_info(media)`)
	if err == nil {
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var dfltValue sql.NullString
			rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
			if name == "person_id" {
				hasPersonID = true
			}
		}
		rows.Close()
	}
	
	if hasPersonID {
		// Old schema detected - perform migration
		
		// Disable foreign key constraints during migration
		_, err = s.DB.Exec(`PRAGMA foreign_keys = OFF;`)
		if err != nil {
			return err
		}
		
		// Start transaction
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		
		// 1. Create person_media junction table
		_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS person_media (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            media_id INTEGER NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
            FOREIGN KEY(media_id) REFERENCES media(id) ON DELETE CASCADE,
            UNIQUE(person_id, media_id)
        );`)
		if err != nil {
			tx.Rollback()
			return err
		}
		
		// 2. Copy existing person_id relationships to person_media
		_, err = tx.Exec(`INSERT OR IGNORE INTO person_media (person_id, media_id) 
			SELECT person_id, id FROM media WHERE person_id IS NOT NULL`)
		if err != nil {
			tx.Rollback()
			return err
		}
		
		// 3. Create new media table without person_id
		_, err = tx.Exec(`CREATE TABLE media_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT,
			description TEXT,
			media_type TEXT DEFAULT 'image',
			mime_type TEXT,
			thumbnail BLOB,
			full_image BLOB,
			external_path TEXT,
			is_external INTEGER DEFAULT 0,
			file_size INTEGER DEFAULT 0,
			date_taken TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`)
		if err != nil {
			tx.Rollback()
			return err
		}
		
		// 4. Copy data (excluding person_id)
		_, err = tx.Exec(`INSERT INTO media_new (id, title, description, media_type, mime_type, thumbnail, full_image, external_path, is_external, file_size, date_taken, created_at, updated_at)
			SELECT id, title, description, media_type, mime_type, thumbnail, full_image, external_path, is_external, file_size, date_taken, created_at, updated_at FROM media`)
		if err != nil {
			tx.Rollback()
			return err
		}
		
		// 5. Drop old table and rename new one
		_, err = tx.Exec(`DROP TABLE media;`)
		if err != nil {
			tx.Rollback()
			return err
		}
		_, err = tx.Exec(`ALTER TABLE media_new RENAME TO media;`)
		if err != nil {
			tx.Rollback()
			return err
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			return err
		}
		
		// Re-enable foreign keys
		_, err = s.DB.Exec(`PRAGMA foreign_keys = ON;`)
		if err != nil {
			return err
		}
		
		// Create indexes
		_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_media_person ON person_media(person_id);`)
		if err != nil {
			return err
		}
		_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_media_media ON person_media(media_id);`)
		if err != nil {
			return err
		}
	} else {
		// New schema - just ensure person_media table exists
		_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS person_media (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            media_id INTEGER NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
            FOREIGN KEY(media_id) REFERENCES media(id) ON DELETE CASCADE,
            UNIQUE(person_id, media_id)
        );`)
		if err != nil {
			return err
		}
		
		_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_media_person ON person_media(person_id);`)
		if err != nil {
			return err
		}
		_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_media_media ON person_media(media_id);`)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// CreatePerson inserts a new person into the DB and sets p.ID.
func (s *Store) CreatePerson(p *Person) error {
	res, err := s.DB.Exec(`INSERT INTO persons(given_name,surname,gender,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.GivenName, p.Surname, p.Gender, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, intFromBool(p.IsLiving), p.Address, p.City, p.State, p.PostalCode, p.Country, p.Email, p.Phone, p.UID, p.Notes)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return nil
}

// UpdatePerson updates an existing person record.
func (s *Store) UpdatePerson(p *Person) error {
	_, err := s.DB.Exec(`UPDATE persons SET given_name=?,surname=?,gender=?,birth_date=?,birth_place=?,death_date=?,death_place=?,is_living=?,address=?,city=?,state=?,postal_code=?,country=?,email=?,phone=?,uid=?,notes=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.GivenName, p.Surname, p.Gender, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, intFromBool(p.IsLiving), p.Address, p.City, p.State, p.PostalCode, p.Country, p.Email, p.Phone, p.UID, p.Notes, p.ID)
	return err
}

// DeletePerson removes a person and cascades to relationships/events via FK.
func (s *Store) DeletePerson(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM persons WHERE id = ?`, id)
	return err
}

// GetPeople returns all people ordered by surname then given name.
func (s *Store) GetPeople() ([]Person, error) {
	rows, err := s.DB.Query(`SELECT id,given_name,surname,gender,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,created_at,updated_at FROM persons ORDER BY surname, given_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var birthDate sql.NullString
		var birthPlace sql.NullString
		var deathDate sql.NullString
		var deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address sql.NullString
		var city sql.NullString
		var state sql.NullString
		var postalCode sql.NullString
		var country sql.NullString
		var email sql.NullString
		var phone sql.NullString
		var uid sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &created, &updated); err != nil {
			return nil, err
		}
		if birthDate.Valid {
			p.BirthDate = birthDate.String
		}
		if birthPlace.Valid {
			p.BirthPlace = birthPlace.String
		}
		if deathDate.Valid {
			p.DeathDate = deathDate.String
		}
		if deathPlace.Valid {
			p.DeathPlace = deathPlace.String
		}
		if isLiving.Valid {
			p.IsLiving = boolFromInt(int(isLiving.Int64))
		}
		if address.Valid {
			p.Address = address.String
		}
		if city.Valid {
			p.City = city.String
		}
		if state.Valid {
			p.State = state.String
		}
		if postalCode.Valid {
			p.PostalCode = postalCode.String
		}
		if country.Valid {
			p.Country = country.String
		}
		if email.Valid {
			p.Email = email.String
		}
		if phone.Valid {
			p.Phone = phone.String
		}
		if uid.Valid {
			p.UID = uid.String
		}
		// do not rely on DB timestamp format - leave zero value when parse fails
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				p.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				p.UpdatedAt = t
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// CountPeople returns the number of persons in DB.
func (s *Store) CountPeople() (int, error) {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM persons`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// GetPersonByID returns a person by ID.
func (s *Store) GetPersonByID(id int64) (*Person, error) {
	var p Person
	var birthDate sql.NullString
	var birthPlace sql.NullString
	var deathDate sql.NullString
	var deathPlace sql.NullString
	var isLiving sql.NullInt64
	var address sql.NullString
	var city sql.NullString
	var state sql.NullString
	var postalCode sql.NullString
	var country sql.NullString
	var email sql.NullString
	var phone sql.NullString
	var uid sql.NullString
	var created sql.NullString
	var updated sql.NullString
	row := s.DB.QueryRow(`SELECT id,given_name,surname,gender,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,created_at,updated_at FROM persons WHERE id = ?`, id)
	if err := row.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &created, &updated); err != nil {
		return nil, err
	}
	if birthDate.Valid {
		p.BirthDate = birthDate.String
	}
	if birthPlace.Valid {
		p.BirthPlace = birthPlace.String
	}
	if deathDate.Valid {
		p.DeathDate = deathDate.String
	}
	if deathPlace.Valid {
		p.DeathPlace = deathPlace.String
	}
	if isLiving.Valid {
		p.IsLiving = boolFromInt(int(isLiving.Int64))
	}
	if address.Valid {
		p.Address = address.String
	}
	if city.Valid {
		p.City = city.String
	}
	if state.Valid {
		p.State = state.String
	}
	if postalCode.Valid {
		p.PostalCode = postalCode.String
	}
	if country.Valid {
		p.Country = country.String
	}
	if email.Valid {
		p.Email = email.String
	}
	if phone.Valid {
		p.Phone = phone.String
	}
	if uid.Valid {
		p.UID = uid.String
	}
	if created.Valid {
		if t, err := time.Parse(time.RFC3339, created.String); err == nil {
			p.CreatedAt = t
		}
	}
	if updated.Valid {
		if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
			p.UpdatedAt = t
		}
	}
	return &p, nil
}

// CreateRelationship creates a bidirectional relationship entry.
// The inverse relationship is automatically created with the appropriate type.
func (s *Store) CreateRelationship(rel *Relationship) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create forward relationship
	res, err := tx.Exec(`INSERT INTO relationships(subject_id,object_id,type,marriage_date,marriage_place,divorce_date,separation_date,end_reason) VALUES (?,?,?,?,?,?,?,?)`, 
		rel.SubjectID, rel.ObjectID, rel.Type, rel.MarriageDate, rel.MarriagePlace, rel.DivorceDate, rel.SeparationDate, rel.EndReason)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	rel.ID = id
	rel.CreatedAt = time.Now()

	// Create inverse relationship (share marriage date/place/divorce for spouse relationships)
	inverseType := getInverseRelationType(rel.Type)
	_, err = tx.Exec(`INSERT INTO relationships(subject_id,object_id,type,marriage_date,marriage_place,divorce_date,separation_date,end_reason) VALUES (?,?,?,?,?,?,?,?)`, 
		rel.ObjectID, rel.SubjectID, inverseType, rel.MarriageDate, rel.MarriagePlace, rel.DivorceDate, rel.SeparationDate, rel.EndReason)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateRelationship updates an existing relationship.
func (s *Store) UpdateRelationship(rel *Relationship) error {
	_, err := s.DB.Exec(`UPDATE relationships 
		SET marriage_date=?, marriage_place=?, divorce_date=?, separation_date=?, end_reason=? 
		WHERE id=?`,
		rel.MarriageDate, rel.MarriagePlace, rel.DivorceDate, rel.SeparationDate, rel.EndReason, rel.ID)
	return err
}

// getInverseRelationType returns the inverse relationship type.
func getInverseRelationType(relType string) string {
	switch relType {
	case "parent":
		return "child"
	case "child":
		return "parent"
	case "spouse", "partner", "cohabitation", "sibling", "other":
		return relType
	default:
		return relType
	}
}

// GetRelationships returns all relationships.
func (s *Store) GetRelationships() ([]Relationship, error) {
	rows, err := s.DB.Query(`SELECT id,subject_id,object_id,type,marriage_date,marriage_place,divorce_date,separation_date,end_reason,created_at FROM relationships`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Relationship
	for rows.Next() {
		var r Relationship
		var marriageDate sql.NullString
		var marriagePlace sql.NullString
		var divorceDate sql.NullString
		var separationDate sql.NullString
		var endReason sql.NullString
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.SubjectID, &r.ObjectID, &r.Type, &marriageDate, &marriagePlace, &divorceDate, &separationDate, &endReason, &created); err != nil {
			return nil, err
		}
		if marriageDate.Valid {
			r.MarriageDate = marriageDate.String
		}
		if marriagePlace.Valid {
			r.MarriagePlace = marriagePlace.String
		}
		if divorceDate.Valid {
			r.DivorceDate = divorceDate.String
		}
		if separationDate.Valid {
			r.SeparationDate = separationDate.String
		}
		if endReason.Valid {
			r.EndReason = endReason.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				r.CreatedAt = t
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// GetRelationshipsBetween returns relationships between two specific people of a certain type
func (s *Store) GetRelationshipsBetween(subjectID, objectID int64, relType string) ([]Relationship, error) {
	query := `SELECT id,subject_id,object_id,type,marriage_date,marriage_place,divorce_date,separation_date,end_reason,created_at 
	          FROM relationships 
	          WHERE subject_id = ? AND object_id = ?`
	
	args := []interface{}{subjectID, objectID}
	
	if relType != "" {
		query += " AND type = ?"
		args = append(args, relType)
	}
	
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var out []Relationship
	for rows.Next() {
		var r Relationship
		var marriageDate sql.NullString
		var marriagePlace sql.NullString
		var divorceDate sql.NullString
		var separationDate sql.NullString
		var endReason sql.NullString
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.SubjectID, &r.ObjectID, &r.Type, &marriageDate, &marriagePlace, &divorceDate, &separationDate, &endReason, &created); err != nil {
			return nil, err
		}
		if marriageDate.Valid {
			r.MarriageDate = marriageDate.String
		}
		if marriagePlace.Valid {
			r.MarriagePlace = marriagePlace.String
		}
		if divorceDate.Valid {
			r.DivorceDate = divorceDate.String
		}
		if separationDate.Valid {
			r.SeparationDate = separationDate.String
		}
		if endReason.Valid {
			r.EndReason = endReason.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				r.CreatedAt = t
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// GetRelatedPeople returns people related to a given person by a specific relationship type.
func (s *Store) GetRelatedPeople(personID int64, relType string) ([]Person, error) {
	rows, err := s.DB.Query(`
		SELECT p.id, p.given_name, p.surname, p.gender, p.birth_date, p.birth_place, p.death_date, p.death_place, p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.created_at, p.updated_at
		FROM persons p
		INNER JOIN relationships r ON r.object_id = p.id
		WHERE r.subject_id = ? AND r.type = ?
	`, personID, relType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var birthDate sql.NullString
		var birthPlace sql.NullString
		var deathDate sql.NullString
		var deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address sql.NullString
		var city sql.NullString
		var state sql.NullString
		var postalCode sql.NullString
		var country sql.NullString
		var email sql.NullString
		var phone sql.NullString
		var uid sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &created, &updated); err != nil {
			return nil, err
		}
		if birthDate.Valid {
			p.BirthDate = birthDate.String
		}
		if birthPlace.Valid {
			p.BirthPlace = birthPlace.String
		}
		if deathDate.Valid {
			p.DeathDate = deathDate.String
		}
		if deathPlace.Valid {
			p.DeathPlace = deathPlace.String
		}
		if isLiving.Valid {
			p.IsLiving = boolFromInt(int(isLiving.Int64))
		}
		if address.Valid {
			p.Address = address.String
		}
		if city.Valid {
			p.City = city.String
		}
		if state.Valid {
			p.State = state.String
		}
		if postalCode.Valid {
			p.PostalCode = postalCode.String
		}
		if country.Valid {
			p.Country = country.String
		}
		if email.Valid {
			p.Email = email.String
		}
		if phone.Valid {
			p.Phone = phone.String
		}
		if uid.Valid {
			p.UID = uid.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				p.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				p.UpdatedAt = t
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// SpouseInfo holds a spouse/partner and their marriage details.
type SpouseInfo struct {
	Spouse         Person // The spouse/partner
	Person         Person // Alias for Spouse (for backward compatibility)
	MarriageDate   string
	MarriagePlace  string
	DivorceDate    string
	SeparationDate string
	EndReason      string
}

// GetSpouses returns all spouses/partners with marriage information.
func (s *Store) GetSpouses(personID int64) ([]SpouseInfo, error) {
	rows, err := s.DB.Query(`
		SELECT p.id, p.given_name, p.surname, p.gender, p.birth_date, p.birth_place, p.death_date, p.death_place, 
		       p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.created_at, p.updated_at, 
		       r.marriage_date, r.marriage_place, r.divorce_date, r.separation_date, r.end_reason
		FROM persons p
		INNER JOIN relationships r ON r.object_id = p.id
		WHERE r.subject_id = ? AND r.type IN ('spouse', 'partner', 'cohabitation', 'other')
		ORDER BY r.marriage_date
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpouseInfo
	for rows.Next() {
		var si SpouseInfo
		var p Person
		var birthDate sql.NullString
		var birthPlace sql.NullString
		var deathDate sql.NullString
		var deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address sql.NullString
		var city sql.NullString
		var state sql.NullString
		var postalCode sql.NullString
		var country sql.NullString
		var email sql.NullString
		var phone sql.NullString
		var uid sql.NullString
		var created sql.NullString
		var updated sql.NullString
		var marriageDate sql.NullString
		var marriagePlace sql.NullString
		var divorceDate sql.NullString
		var separationDate sql.NullString
		var endReason sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace, 
			&isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &created, &updated, &marriageDate, &marriagePlace, &divorceDate, &separationDate, &endReason); err != nil {
			return nil, err
		}
		if birthDate.Valid {
			p.BirthDate = birthDate.String
		}
		if birthPlace.Valid {
			p.BirthPlace = birthPlace.String
		}
		if deathDate.Valid {
			p.DeathDate = deathDate.String
		}
		if deathPlace.Valid {
			p.DeathPlace = deathPlace.String
		}
		if isLiving.Valid {
			p.IsLiving = boolFromInt(int(isLiving.Int64))
		}
		if address.Valid {
			p.Address = address.String
		}
		if city.Valid {
			p.City = city.String
		}
		if state.Valid {
			p.State = state.String
		}
		if postalCode.Valid {
			p.PostalCode = postalCode.String
		}
		if country.Valid {
			p.Country = country.String
		}
		if email.Valid {
			p.Email = email.String
		}
		if phone.Valid {
			p.Phone = phone.String
		}
		if uid.Valid {
			p.UID = uid.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				p.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				p.UpdatedAt = t
			}
		}
		if marriageDate.Valid {
			si.MarriageDate = marriageDate.String
		}
		if marriagePlace.Valid {
			si.MarriagePlace = marriagePlace.String
		}
		if divorceDate.Valid {
			si.DivorceDate = divorceDate.String
		}
		if separationDate.Valid {
			si.SeparationDate = separationDate.String
		}
		if endReason.Valid {
			si.EndReason = endReason.String
		}
		si.Person = p
		si.Spouse = p // Set both for compatibility
		out = append(out, si)
	}
	return out, nil
}

// GetChildrenOfParents returns all children who have both specified people as parents.
func (s *Store) GetChildrenOfParents(parent1ID, parent2ID int64) ([]Person, error) {
	// Find children who have relationships to both parents
	rows, err := s.DB.Query(`
		SELECT DISTINCT p.id, p.given_name, p.surname, p.gender, p.birth_date, p.birth_place, 
		       p.death_date, p.death_place, p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.created_at, p.updated_at
		FROM persons p
		WHERE EXISTS (
			SELECT 1 FROM relationships r1 
			WHERE r1.subject_id = p.id AND r1.object_id = ? AND r1.type = 'parent'
		)
		AND EXISTS (
			SELECT 1 FROM relationships r2 
			WHERE r2.subject_id = p.id AND r2.object_id = ? AND r2.type = 'parent'
		)
	`, parent1ID, parent2ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Person
	for rows.Next() {
		var p Person
		var birthDate sql.NullString
		var birthPlace sql.NullString
		var deathDate sql.NullString
		var deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address sql.NullString
		var city sql.NullString
		var state sql.NullString
		var postalCode sql.NullString
		var country sql.NullString
		var email sql.NullString
		var phone sql.NullString
		var uid sql.NullString
		var created sql.NullString
		var updated sql.NullString
		
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, 
			&deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &created, &updated); err != nil {
			return nil, err
		}
		
		if birthDate.Valid {
			p.BirthDate = birthDate.String
		}
		if birthPlace.Valid {
			p.BirthPlace = birthPlace.String
		}
		if deathDate.Valid {
			p.DeathDate = deathDate.String
		}
		if deathPlace.Valid {
			p.DeathPlace = deathPlace.String
		}
		if isLiving.Valid {
			p.IsLiving = boolFromInt(int(isLiving.Int64))
		}
		if address.Valid {
			p.Address = address.String
		}
		if city.Valid {
			p.City = city.String
		}
		if state.Valid {
			p.State = state.String
		}
		if postalCode.Valid {
			p.PostalCode = postalCode.String
		}
		if country.Valid {
			p.Country = country.String
		}
		if email.Valid {
			p.Email = email.String
		}
		if phone.Valid {
			p.Phone = phone.String
		}
		if uid.Valid {
			p.UID = uid.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				p.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				p.UpdatedAt = t
			}
		}
		
		out = append(out, p)
	}
	return out, nil
}

// CreateEvent inserts an event record (birth/death/place/note) for a person.
func (s *Store) CreateEvent(e *Event) error {
	res, err := s.DB.Exec(`INSERT INTO events(person_id,type,date,place,note) VALUES (?,?,?,?,?)`, e.PersonID, e.Type, e.Date, e.Place, e.Note)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	e.ID = id
	e.CreatedAt = time.Now()
	return nil
}

// CreateIdentifier inserts an identifier (e.g., UID or REFN) for a person.
func (s *Store) CreateIdentifier(idt *Identifier) error {
	res, err := s.DB.Exec(`INSERT INTO identifiers(person_id,tag,value) VALUES (?,?,?)`, idt.PersonID, idt.Tag, idt.Value)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	idt.ID = id
	idt.CreatedAt = time.Now()
	return nil
}

// GetEvents returns events for a person.
func (s *Store) GetEvents(personID int64) ([]Event, error) {
	rows, err := s.DB.Query(`SELECT id,person_id,type,date,place,note,created_at FROM events WHERE person_id = ?`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var created sql.NullString
		if err := rows.Scan(&e.ID, &e.PersonID, &e.Type, &e.Date, &e.Place, &e.Note, &created); err != nil {
			return nil, err
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				e.CreatedAt = t
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// ========================================
// Media Operations
// ========================================

// CreateMedia inserts a new media record (without linking to any person yet)
func (s *Store) CreateMedia(m *Media) error {
	res, err := s.DB.Exec(`INSERT INTO media (title, description, media_type, mime_type, thumbnail, full_image, external_path, is_external, file_size, date_taken) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Title, m.Description, m.MediaType, m.MimeType, m.Thumbnail, m.FullImage, m.ExternalPath, intFromBool(m.IsExternal), m.FileSize, m.DateTaken)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

// LinkMediaToPerson creates a link between a media item and a person
func (s *Store) LinkMediaToPerson(mediaID, personID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO person_media (person_id, media_id) VALUES (?, ?)`, personID, mediaID)
	return err
}

// UnlinkMediaFromPerson removes a link between a media item and a person
func (s *Store) UnlinkMediaFromPerson(mediaID, personID int64) error {
	_, err := s.DB.Exec(`DELETE FROM person_media WHERE person_id=? AND media_id=?`, personID, mediaID)
	return err
}

// GetLinkedPeopleForMedia returns the IDs of all people linked to a media item
func (s *Store) GetLinkedPeopleForMedia(mediaID int64) ([]int64, error) {
	rows, err := s.DB.Query(`SELECT person_id FROM person_media WHERE media_id=?`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var personIDs []int64
	for rows.Next() {
		var personID int64
		if err := rows.Scan(&personID); err != nil {
			return nil, err
		}
		personIDs = append(personIDs, personID)
	}
	return personIDs, nil
}

// GetMediaByID retrieves a single media record by ID
func (s *Store) GetMediaByID(id int64) (*Media, error) {
	var m Media
	var created, updated, dateT sql.NullString
	var isExt sql.NullInt64
	err := s.DB.QueryRow(`SELECT id, title, description, media_type, mime_type, thumbnail, full_image, external_path, is_external, file_size, date_taken, created_at, updated_at FROM media WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &m.Description, &m.MediaType, &m.MimeType, &m.Thumbnail, &m.FullImage, &m.ExternalPath, &isExt, &m.FileSize, &dateT, &created, &updated)
	if err != nil {
		return nil, err
	}
	m.IsExternal = isExt.Int64 != 0
	if dateT.Valid {
		m.DateTaken = dateT.String
	}
	if created.Valid {
		if t, err := time.Parse(time.RFC3339, created.String); err == nil {
			m.CreatedAt = t
		}
	}
	if updated.Valid {
		if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
			m.UpdatedAt = t
		}
	}
	
	// Load linked people
	m.LinkedPeople, _ = s.GetLinkedPeopleForMedia(m.ID)
	
	return &m, nil
}

// GetMediaForPerson retrieves all media for a person
func (s *Store) GetMediaForPerson(personID int64) ([]Media, error) {
	rows, err := s.DB.Query(`
		SELECT m.id, m.title, m.description, m.media_type, m.mime_type, m.external_path, m.is_external, m.file_size, m.date_taken, m.created_at, m.updated_at 
		FROM media m
		JOIN person_media pm ON m.id = pm.media_id
		WHERE pm.person_id=? 
		ORDER BY m.created_at DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var out []Media
	for rows.Next() {
		var m Media
		var created, updated, dateT sql.NullString
		var isExt sql.NullInt64
		// Note: Not loading thumbnail and full_image here for performance (they're large BLOBs)
		// Use GetMediaByID to load the full data including images
		if err := rows.Scan(&m.ID, &m.Title, &m.Description, &m.MediaType, &m.MimeType, &m.ExternalPath, &isExt, &m.FileSize, &dateT, &created, &updated); err != nil {
			return nil, err
		}
		m.IsExternal = isExt.Int64 != 0
		if dateT.Valid {
			m.DateTaken = dateT.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				m.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				m.UpdatedAt = t
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// UpdateMedia updates an existing media record
func (s *Store) UpdateMedia(m *Media) error {
	_, err := s.DB.Exec(`UPDATE media SET title=?, description=?, media_type=?, mime_type=?, thumbnail=?, full_image=?, external_path=?, is_external=?, file_size=?, date_taken=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		m.Title, m.Description, m.MediaType, m.MimeType, m.Thumbnail, m.FullImage, m.ExternalPath, intFromBool(m.IsExternal), m.FileSize, m.DateTaken, m.ID)
	return err
}

// DeleteMedia removes a media record (and all links to people via CASCADE)
func (s *Store) DeleteMedia(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM media WHERE id=?`, id)
	return err
}

// GetMediaLinkCount returns the number of people linked to a media item
func (s *Store) GetMediaLinkCount(mediaID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM person_media WHERE media_id=?`, mediaID).Scan(&count)
	return count, err
}

// GetMediaCount returns the total count of media items in the database
func (s *Store) GetMediaCount() (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&count)
	return count, err
}

// GetAllMedia retrieves all media items ordered by creation date
func (s *Store) GetAllMedia() ([]Media, error) {
	rows, err := s.DB.Query(`
		SELECT id, title, description, media_type, mime_type, external_path, is_external, file_size, date_taken, created_at, updated_at 
		FROM media 
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var out []Media
	for rows.Next() {
		var m Media
		var created, updated, dateT sql.NullString
		var isExt sql.NullInt64
		if err := rows.Scan(&m.ID, &m.Title, &m.Description, &m.MediaType, &m.MimeType, &m.ExternalPath, &isExt, &m.FileSize, &dateT, &created, &updated); err != nil {
			return nil, err
		}
		m.IsExternal = isExt.Int64 != 0
		if dateT.Valid {
			m.DateTaken = dateT.String
		}
		if created.Valid {
			if t, err := time.Parse(time.RFC3339, created.String); err == nil {
				m.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
				m.UpdatedAt = t
			}
		}
		out = append(out, m)
	}
	return out, nil
}
