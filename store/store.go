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
	// Allow a few concurrent connections for better UI responsiveness with background operations
	// SQLite with WAL mode can handle multiple readers + one writer
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

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
            preferred_name TEXT,
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
            bookmarked INTEGER DEFAULT 0,
            last_accessed DATETIME,
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
            title TEXT NOT NULL,
            author TEXT,
            publication TEXT,
            repository TEXT,
            call_number TEXT,
            notes TEXT,
            source_type TEXT DEFAULT 'other',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS citations (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            source_id INTEGER NOT NULL,
            person_id INTEGER NOT NULL,
            citation_detail TEXT,
            transcription TEXT,
            confidence TEXT DEFAULT 'medium',
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
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
		`CREATE TABLE IF NOT EXISTS validated_items (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            item_type TEXT NOT NULL,
            person_id INTEGER NOT NULL,
            related_person_id INTEGER,
            conflict_type TEXT NOT NULL,
            validation_note TEXT,
            reviewed_by TEXT,
            validated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
            UNIQUE(item_type, person_id, related_person_id, conflict_type)
        );`,
		`CREATE TABLE IF NOT EXISTS research_todos (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            description TEXT NOT NULL,
            priority TEXT DEFAULT 'medium',
            status TEXT DEFAULT 'pending',
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            completed_at DATETIME,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS research_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER,
            search_date DATETIME NOT NULL,
            repository TEXT NOT NULL,
            record_type TEXT,
            search_goal TEXT NOT NULL,
            results TEXT,
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS research_log_people (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            research_log_id INTEGER NOT NULL,
            person_id INTEGER NOT NULL,
            FOREIGN KEY(research_log_id) REFERENCES research_logs(id) ON DELETE CASCADE,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
            UNIQUE(research_log_id, person_id)
        );`,
		`CREATE INDEX IF NOT EXISTS idx_person_name ON persons(surname, given_name);`,
		`CREATE INDEX IF NOT EXISTS idx_relationship_subject ON relationships(subject_id);`,
		`CREATE INDEX IF NOT EXISTS idx_relationship_object ON relationships(object_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_person ON events(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_person_media_person ON person_media(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_person_media_media ON person_media(media_id);`,
		`CREATE INDEX IF NOT EXISTS idx_validated_items_person ON validated_items(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_validated_items_type ON validated_items(item_type, conflict_type);`,
		`CREATE INDEX IF NOT EXISTS idx_research_todos_person ON research_todos(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_research_todos_status ON research_todos(status);`,
		`CREATE INDEX IF NOT EXISTS idx_research_logs_person ON research_logs(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_research_logs_date ON research_logs(search_date);`,
		`CREATE INDEX IF NOT EXISTS idx_research_logs_repository ON research_logs(repository);`,
		`CREATE INDEX IF NOT EXISTS idx_research_log_people_log ON research_log_people(research_log_id);`,
		`CREATE INDEX IF NOT EXISTS idx_research_log_people_person ON research_log_people(person_id);`,
		`CREATE INDEX IF NOT EXISTS idx_citations_source ON citations(source_id);`,
		`CREATE INDEX IF NOT EXISTS idx_citations_person ON citations(person_id);`,
		`CREATE TABLE IF NOT EXISTS saved_searches (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            criteria_json TEXT NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE INDEX IF NOT EXISTS idx_saved_searches_name ON saved_searches(name);`,
		`CREATE TABLE IF NOT EXISTS place_geocodes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            place_name TEXT NOT NULL UNIQUE,
            latitude REAL,
            longitude REAL,
            country TEXT,
            state_province TEXT,
            city TEXT,
            geocode_date DATETIME DEFAULT CURRENT_TIMESTAMP,
            geocode_status TEXT DEFAULT 'pending',
            notes TEXT
        );`,
		`CREATE INDEX IF NOT EXISTS idx_place_geocodes_name ON place_geocodes(place_name);`,
		`CREATE INDEX IF NOT EXISTS idx_place_geocodes_status ON place_geocodes(geocode_status);`,
		`CREATE TABLE IF NOT EXISTS project_notes (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            notes TEXT DEFAULT '',
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`INSERT OR IGNORE INTO project_notes (id, notes) VALUES (1, '');`,
		`CREATE TABLE IF NOT EXISTS settings (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            last_backup_date DATETIME,
            backup_reminder_enabled INTEGER DEFAULT 1,
            backup_reminder_days INTEGER DEFAULT 7,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`INSERT OR IGNORE INTO settings (id) VALUES (1);`,
		`CREATE TABLE IF NOT EXISTS alternate_names (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL,
            name_type TEXT DEFAULT 'spelling',
            given_name TEXT,
            surname TEXT,
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_alternate_names_person ON alternate_names(person_id);`,
		`CREATE TABLE IF NOT EXISTS alternate_places (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            historical_name TEXT NOT NULL,
            current_name TEXT NOT NULL,
            year_changed INTEGER,
            notes TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE INDEX IF NOT EXISTS idx_alternate_places_historical ON alternate_places(historical_name);`,
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
		`ALTER TABLE persons ADD COLUMN bookmarked INTEGER DEFAULT 0;`,
		`ALTER TABLE persons ADD COLUMN last_accessed DATETIME;`,
		`ALTER TABLE sources ADD COLUMN author TEXT;`,
		`ALTER TABLE sources ADD COLUMN publication TEXT;`,
		`ALTER TABLE sources ADD COLUMN repository TEXT;`,
		`ALTER TABLE sources ADD COLUMN call_number TEXT;`,
		`ALTER TABLE sources ADD COLUMN notes TEXT;`,
		`ALTER TABLE sources ADD COLUMN source_type TEXT DEFAULT 'other';`,
		`ALTER TABLE sources ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP;`,
		`ALTER TABLE persons ADD COLUMN preferred_name TEXT;`,
		`ALTER TABLE events ADD COLUMN date_end TEXT;`,
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

	// Add indexes for bookmarked and last_accessed columns (after migrations have added the columns)
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_bookmarked ON persons(bookmarked);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_person_last_accessed ON persons(last_accessed);`)
	if err != nil {
		return err
	}

	// Create citations table if it doesn't exist (for databases created before this feature)
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS citations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL,
		person_id INTEGER NOT NULL,
		citation_detail TEXT,
		transcription TEXT,
		confidence TEXT DEFAULT 'medium',
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE,
		FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
	);`)
	if err != nil {
		return err
	}

	// Add indexes for citations table
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_citations_source ON citations(source_id);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_citations_person ON citations(person_id);`)
	if err != nil {
		return err
	}

	// Create research_logs table if it doesn't exist
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS research_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id INTEGER,
		search_date DATETIME NOT NULL,
		repository TEXT NOT NULL,
		record_type TEXT,
		search_goal TEXT NOT NULL,
		results TEXT,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}

	// Add indexes for research_logs table
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_research_logs_person ON research_logs(person_id);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_research_logs_date ON research_logs(search_date);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_research_logs_repository ON research_logs(repository);`)
	if err != nil {
		return err
	}

	// Create research_log_people junction table if it doesn't exist
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS research_log_people (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		research_log_id INTEGER NOT NULL,
		person_id INTEGER NOT NULL,
		FOREIGN KEY(research_log_id) REFERENCES research_logs(id) ON DELETE CASCADE,
		FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE,
		UNIQUE(research_log_id, person_id)
	);`)
	if err != nil {
		return err
	}

	// Add indexes for research_log_people table
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_research_log_people_log ON research_log_people(research_log_id);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_research_log_people_person ON research_log_people(person_id);`)
	if err != nil {
		return err
	}

	// Migrate existing person_id data from research_logs to research_log_people
	_, err = s.DB.Exec(`
		INSERT OR IGNORE INTO research_log_people (research_log_id, person_id)
		SELECT id, person_id FROM research_logs WHERE person_id IS NOT NULL
	`)
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

	// Create project_notes table if it doesn't exist (v1.5.1+)
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS project_notes (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		notes TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT OR IGNORE INTO project_notes (id, notes) VALUES (1, '');`)
	if err != nil {
		return err
	}

	// Create alternate_names table if it doesn't exist (v1.5.3+)
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS alternate_names (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id INTEGER NOT NULL,
		name_type TEXT DEFAULT 'spelling',
		given_name TEXT,
		surname TEXT,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(person_id) REFERENCES persons(id) ON DELETE CASCADE
	);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_alternate_names_person ON alternate_names(person_id);`)
	if err != nil {
		return err
	}

	// Create alternate_places table if it doesn't exist (v1.7+)
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS alternate_places (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		historical_name TEXT NOT NULL,
		current_name TEXT NOT NULL,
		year_changed INTEGER,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_alternate_places_historical ON alternate_places(historical_name);`)
	if err != nil {
		return err
	}

	// Create settings table if it doesn't exist (v1.6.0+)
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		last_backup_date DATETIME,
		backup_reminder_enabled INTEGER DEFAULT 1,
		backup_reminder_days INTEGER DEFAULT 7,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}

	// Initialize settings with default row
	_, err = s.DB.Exec(`INSERT OR IGNORE INTO settings (id) VALUES (1);`)
	if err != nil {
		return err
	}

	// Performance indexes (v1.6.0+) - speed up common queries
	performanceIndexes := []string{
		// Date-based queries (timeline reports, date range filtering)
		`CREATE INDEX IF NOT EXISTS idx_person_birth_date ON persons(birth_date);`,
		`CREATE INDEX IF NOT EXISTS idx_person_death_date ON persons(death_date);`,
		`CREATE INDEX IF NOT EXISTS idx_person_birth_year ON persons(birth_year);`,
		`CREATE INDEX IF NOT EXISTS idx_person_death_year ON persons(death_year);`,

		// Place-based queries (autocomplete, geographic reports)
		`CREATE INDEX IF NOT EXISTS idx_person_birth_place ON persons(birth_place);`,
		`CREATE INDEX IF NOT EXISTS idx_person_death_place ON persons(death_place);`,
		`CREATE INDEX IF NOT EXISTS idx_relationship_marriage_place ON relationships(marriage_place);`,

		// Common filters
		`CREATE INDEX IF NOT EXISTS idx_person_is_living ON persons(is_living);`,
		`CREATE INDEX IF NOT EXISTS idx_person_gender ON persons(gender);`,
		`CREATE INDEX IF NOT EXISTS idx_person_uid ON persons(uid);`,

		// Preferred name for search
		`CREATE INDEX IF NOT EXISTS idx_person_preferred_name ON persons(preferred_name);`,

		// Relationship type filtering
		`CREATE INDEX IF NOT EXISTS idx_relationship_type ON relationships(type);`,
	}

	for _, indexSQL := range performanceIndexes {
		if _, err := s.DB.Exec(indexSQL); err != nil {
			// Log but don't fail - indexes are performance optimization
			fmt.Printf("Warning: Could not create performance index: %v\n", err)
		}
	}

	// Optimize query planner statistics after creating indexes
	s.DB.Exec(`PRAGMA optimize;`)
	
	// Create FTS5 virtual table for full-text search (v1.8)
	// Check if FTS table exists and has correct schema
	var tableName string
	err = s.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='persons_fts'`).Scan(&tableName)
	
	if err == nil {
		// Table exists - check if it has the new columns
		var hasMarriagePlaces bool
		err = s.DB.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='persons_fts'`).Scan(&tableName)
		hasMarriagePlaces = strings.Contains(tableName, "marriage_places")
		
		if !hasMarriagePlaces {
			// Old schema - drop and recreate
			fmt.Println("Upgrading full-text search index with marriage places and spouse names...")
			s.DB.Exec(`DROP TABLE IF EXISTS persons_fts;`)
		}
	}
	
	// Create FTS5 table with full schema
	_, err = s.DB.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS persons_fts USING fts5(
			person_id UNINDEXED,
			given_name,
			surname,
			preferred_name,
			notes,
			birth_place,
			death_place,
			address,
			city,
			state,
			country,
			uid,
			marriage_places,
			spouse_names,
			tokenize = 'porter unicode61'
		);
	`)
	if err != nil {
		fmt.Printf("Warning: Could not create FTS5 table: %v\n", err)
	}
	
	// Populate FTS table with existing data (idempotent - replaces all)
	err = s.RebuildFullTextIndex()
	if err != nil {
		fmt.Printf("Warning: Could not populate FTS5 table: %v\n", err)
	}

	// Create audit_log table if it doesn't exist
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		operation_type TEXT NOT NULL,
		description TEXT NOT NULL,
		person_id INTEGER,
		related_person_id INTEGER
	);`)
	if err != nil {
		return err
	}

	// Add index for audit_log table
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);`)
	if err != nil {
		return err
	}

	return nil
}

// CreatePerson inserts a new person into the DB and sets p.ID.
func (s *Store) CreatePerson(p *Person) error {
	res, err := s.DB.Exec(`INSERT INTO persons(given_name,surname,preferred_name,gender,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.GivenName, p.Surname, p.PreferredName, p.Gender, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, intFromBool(p.IsLiving), p.Address, p.City, p.State, p.PostalCode, p.Country, p.Email, p.Phone, p.UID, p.Notes)
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
	
	// Update FTS index
	s.UpdateFullTextIndexForPerson(p.ID)
	
	return nil
}

// CreatePersonWithID inserts a person with a specific ID (for undo/redo)
func (s *Store) CreatePersonWithID(p *Person) error {
	_, err := s.DB.Exec(`INSERT INTO persons(id,given_name,surname,preferred_name,gender,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,bookmarked,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.GivenName, p.Surname, p.PreferredName, p.Gender, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, intFromBool(p.IsLiving), p.Address, p.City, p.State, p.PostalCode, p.Country, p.Email, p.Phone, p.UID, p.Notes, intFromBool(p.Bookmarked), p.CreatedAt, p.UpdatedAt)
	return err
}

// UpdatePerson updates an existing person record.
func (s *Store) UpdatePerson(p *Person) error {
	_, err := s.DB.Exec(`UPDATE persons SET given_name=?,surname=?,preferred_name=?,gender=?,birth_date=?,birth_place=?,death_date=?,death_place=?,is_living=?,address=?,city=?,state=?,postal_code=?,country=?,email=?,phone=?,uid=?,notes=?,bookmarked=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.GivenName, p.Surname, p.PreferredName, p.Gender, p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, intFromBool(p.IsLiving), p.Address, p.City, p.State, p.PostalCode, p.Country, p.Email, p.Phone, p.UID, p.Notes, intFromBool(p.Bookmarked), p.ID)
	
	if err == nil {
		// Update FTS index
		s.UpdateFullTextIndexForPerson(p.ID)
	}
	
	return err
}

// DeletePerson removes a person and cascades to relationships/events via FK.
func (s *Store) DeletePerson(id int64) error {
	// Delete from FTS index first
	s.DeleteFullTextIndexForPerson(id)
	
	_, err := s.DB.Exec(`DELETE FROM persons WHERE id = ?`, id)
	return err
}

// GetPeople returns all people ordered by surname then given name.
func (s *Store) GetPeople() ([]Person, error) {
	rows, err := s.DB.Query(`SELECT id,given_name,surname,preferred_name,gender,birth_year,death_year,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,bookmarked,last_accessed,created_at,updated_at FROM persons ORDER BY surname, given_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var preferredName sql.NullString
		var birthYear sql.NullInt64 // Legacy column - read but ignore
		var deathYear sql.NullInt64 // Legacy column - read but ignore
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &p.Gender, &birthYear, &deathYear, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
			return nil, err
		}
		if preferredName.Valid {
			p.PreferredName = preferredName.String
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, err
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
func (s *Store) GetAllPeople() ([]Person, error) {
	rows, err := s.DB.Query(`
		SELECT id, given_name, surname, preferred_name, birth_year, death_year, 
		       birth_date, birth_place, death_date, death_place, 
		       is_living, gender, address, bookmarked
		FROM persons
		ORDER BY surname, given_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		var preferredName sql.NullString
		var birthYear sql.NullInt64
		var deathYear sql.NullInt64
		var birthDate sql.NullString
		var birthPlace sql.NullString
		var deathDate sql.NullString
		var deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address sql.NullString

		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &birthYear, &deathYear,
			&birthDate, &birthPlace, &deathDate, &deathPlace,
			&isLiving, &p.Gender, &address, &p.Bookmarked); err != nil {
			return nil, err
		}

		if preferredName.Valid {
			p.PreferredName = preferredName.String
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
			p.IsLiving = isLiving.Int64 != 0
		}
		if address.Valid {
			p.Address = address.String
		}

		people = append(people, p)
	}

	return people, rows.Err()
}

func (s *Store) GetPersonByID(id int64) (*Person, error) {
	var p Person
	var preferredName sql.NullString
	var birthYear sql.NullInt64 // Legacy column - read but ignore
	var deathYear sql.NullInt64 // Legacy column - read but ignore
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
	var bookmarked sql.NullInt64
	var lastAccessed sql.NullString
	var created sql.NullString
	var updated sql.NullString
	row := s.DB.QueryRow(`SELECT id,given_name,surname,preferred_name,gender,birth_year,death_year,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,bookmarked,last_accessed,created_at,updated_at FROM persons WHERE id = ?`, id)
	if err := row.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &p.Gender, &birthYear, &deathYear, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
		return nil, err
	}
	if preferredName.Valid {
		p.PreferredName = preferredName.String
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
	if bookmarked.Valid {
		p.Bookmarked = boolFromInt(int(bookmarked.Int64))
	}
	if lastAccessed.Valid {
		if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
			p.LastAccessed = &t
		}
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

// SetBookmarked sets the bookmarked status for a person.
func (s *Store) SetBookmarked(personID int64, bookmarked bool) error {
	_, err := s.DB.Exec(`UPDATE persons SET bookmarked=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, intFromBool(bookmarked), personID)
	return err
}

// TrackPersonAccess updates the last_accessed timestamp for a person.
func (s *Store) TrackPersonAccess(personID int64) error {
	_, err := s.DB.Exec(`UPDATE persons SET last_accessed=CURRENT_TIMESTAMP WHERE id=?`, personID)
	return err
}

// GetRecentPeople returns the most recently accessed people (up to limit).
func (s *Store) GetRecentPeople(limit int) ([]Person, error) {
	rows, err := s.DB.Query(`SELECT id,given_name,surname,preferred_name,gender,birth_year,death_year,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,bookmarked,last_accessed,created_at,updated_at FROM persons WHERE last_accessed IS NOT NULL ORDER BY last_accessed DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var preferredName sql.NullString
		var birthYear sql.NullInt64 // Legacy column - read but ignore
		var deathYear sql.NullInt64 // Legacy column - read but ignore
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &p.Gender, &birthYear, &deathYear, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
			return nil, err
		}
		if preferredName.Valid {
			p.PreferredName = preferredName.String
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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

// GetBookmarkedPeople returns all bookmarked people.
func (s *Store) GetBookmarkedPeople() ([]Person, error) {
	rows, err := s.DB.Query(`SELECT id,given_name,surname,preferred_name,gender,birth_year,death_year,birth_date,birth_place,death_date,death_place,is_living,address,city,state,postal_code,country,email,phone,uid,notes,bookmarked,last_accessed,created_at,updated_at FROM persons WHERE bookmarked=1 ORDER BY surname, given_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var preferredName sql.NullString
		var birthYear sql.NullInt64 // Legacy column - read but ignore
		var deathYear sql.NullInt64 // Legacy column - read but ignore
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &p.Gender, &birthYear, &deathYear, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
			return nil, err
		}
		if preferredName.Valid {
			p.PreferredName = preferredName.String
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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

	if err := tx.Commit(); err != nil {
		return err
	}
	
	// Update FTS index for both people (marriage places and spouse names changed)
	s.UpdateFullTextIndexForPerson(rel.SubjectID)
	s.UpdateFullTextIndexForPerson(rel.ObjectID)
	
	return nil
}

// UpdateRelationship updates an existing relationship.
func (s *Store) UpdateRelationship(rel *Relationship) error {
	// Get the subject_id and object_id before updating (for FTS index update)
	var subjectID, objectID int64
	var relType string
	err := s.DB.QueryRow(`SELECT subject_id, object_id, type FROM relationships WHERE id = ?`, rel.ID).Scan(&subjectID, &objectID, &relType)
	if err != nil {
		return err
	}
	
	// Update the relationship
	_, err = s.DB.Exec(`UPDATE relationships 
		SET marriage_date=?, marriage_place=?, divorce_date=?, separation_date=?, end_reason=? 
		WHERE id=?`,
		rel.MarriageDate, rel.MarriagePlace, rel.DivorceDate, rel.SeparationDate, rel.EndReason, rel.ID)
	
	if err == nil {
		// Also update the inverse relationship with same data
		inverseType := getInverseRelationType(relType)
		s.DB.Exec(`UPDATE relationships 
			SET marriage_date=?, marriage_place=?, divorce_date=?, separation_date=?, end_reason=? 
			WHERE subject_id=? AND object_id=? AND type=?`,
			rel.MarriageDate, rel.MarriagePlace, rel.DivorceDate, rel.SeparationDate, rel.EndReason,
			objectID, subjectID, inverseType)
		
		// Update FTS index for both people
		s.UpdateFullTextIndexForPerson(subjectID)
		s.UpdateFullTextIndexForPerson(objectID)
	}
	
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

// GetRelationshipsForPerson returns all relationships for a specific person (either as subject or object).
func (s *Store) GetRelationshipsForPerson(personID int64) ([]Relationship, error) {
	rows, err := s.DB.Query(`SELECT id,subject_id,object_id,type,marriage_date,marriage_place,divorce_date,separation_date,end_reason,created_at FROM relationships WHERE subject_id = ? OR object_id = ?`, personID, personID)
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
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRelationship deletes a specific relationship.
func (s *Store) DeleteRelationship(subjectID, objectID int64, relType string) error {
	_, err := s.DB.Exec(`DELETE FROM relationships WHERE subject_id = ? AND object_id = ? AND type = ?`, subjectID, objectID, relType)
	
	if err == nil {
		// Update FTS index for both people (marriage places and spouse names may have changed)
		s.UpdateFullTextIndexForPerson(subjectID)
		s.UpdateFullTextIndexForPerson(objectID)
	}
	
	return err
}

// RemoveRelationship is an alias for DeleteRelationship (for undo/redo clarity)
func (s *Store) RemoveRelationship(subjectID, objectID int64, relType string) error {
	return s.DeleteRelationship(subjectID, objectID, relType)
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

// GetRelationshipCount returns the total number of relationships in the database
func (s *Store) GetRelationshipCount() (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM relationships`).Scan(&count)
	return count, err
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
		SELECT p.id, p.given_name, p.surname, p.gender, p.birth_date, p.birth_place, p.death_date, p.death_place, p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.bookmarked, p.last_accessed, p.created_at, p.updated_at
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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
		       p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.bookmarked, p.last_accessed, p.created_at, p.updated_at, 
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString
		var marriageDate sql.NullString
		var marriagePlace sql.NullString
		var divorceDate sql.NullString
		var separationDate sql.NullString
		var endReason sql.NullString
		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace, &deathDate, &deathPlace,
			&isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated, &marriageDate, &marriagePlace, &divorceDate, &separationDate, &endReason); err != nil {
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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
		       p.death_date, p.death_place, p.is_living, p.address, p.city, p.state, p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, p.bookmarked, p.last_accessed, p.created_at, p.updated_at
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
		var bookmarked sql.NullInt64
		var lastAccessed sql.NullString
		var created sql.NullString
		var updated sql.NullString

		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &p.Gender, &birthDate, &birthPlace,
			&deathDate, &deathPlace, &isLiving, &address, &city, &state, &postalCode, &country, &email, &phone, &uid, &p.Notes, &bookmarked, &lastAccessed, &created, &updated); err != nil {
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
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			if t, err := time.Parse(time.RFC3339, lastAccessed.String); err == nil {
				p.LastAccessed = &t
			}
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
	res, err := s.DB.Exec(`INSERT INTO events(person_id,type,date,date_end,place,note) VALUES (?,?,?,?,?,?)`, e.PersonID, e.Type, e.Date, e.DateEnd, e.Place, e.Note)
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
	rows, err := s.DB.Query(`SELECT id,person_id,type,date,date_end,place,note,created_at FROM events WHERE person_id = ?`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var dateEnd, created sql.NullString
		if err := rows.Scan(&e.ID, &e.PersonID, &e.Type, &e.Date, &dateEnd, &e.Place, &e.Note, &created); err != nil {
			return nil, err
		}
		if dateEnd.Valid {
			e.DateEnd = dateEnd.String
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

// UpdateEvent updates an existing event record.
func (s *Store) UpdateEvent(e *Event) error {
	_, err := s.DB.Exec(`UPDATE events SET type=?, date=?, date_end=?, place=?, note=? WHERE id=?`,
		e.Type, e.Date, e.DateEnd, e.Place, e.Note, e.ID)
	return err
}

// DeleteEvent removes an event record.
func (s *Store) DeleteEvent(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM events WHERE id=?`, id)
	return err
}

// CreateAuditLogEntry inserts a persistent audit log record.
func (s *Store) CreateAuditLogEntry(e *AuditLogEntry) error {
	res, err := s.DB.Exec(`
		INSERT INTO audit_log (timestamp, operation_type, description, person_id, related_person_id)
		VALUES (?, ?, ?, ?, ?)
	`, e.Timestamp, e.OperationType, e.Description, e.PersonID, e.RelatedPersonID)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	e.ID = id
	return nil
}

// GetAllAuditLogEntries retrieves all audit log entries, newest first.
func (s *Store) GetAllAuditLogEntries() ([]AuditLogEntry, error) {
	rows, err := s.DB.Query(`
		SELECT id, timestamp, operation_type, description, person_id, related_person_id
		FROM audit_log
		ORDER BY timestamp DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		var personID, relatedPersonID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.OperationType, &e.Description, &personID, &relatedPersonID); err != nil {
			return nil, err
		}
		if personID.Valid {
			e.PersonID = &personID.Int64
		}
		if relatedPersonID.Valid {
			e.RelatedPersonID = &relatedPersonID.Int64
		}
		out = append(out, e)
	}
	return out, rows.Err()
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

// =============== Validated Items Functions ===============

// MarkAsReviewed marks a conflict or duplicate as reviewed/validated.
func (s *Store) MarkAsReviewed(itemType string, personID int64, relatedPersonID *int64, conflictType, note, reviewedBy string) error {
	var relID sql.NullInt64
	if relatedPersonID != nil {
		relID.Int64 = *relatedPersonID
		relID.Valid = true
	}

	_, err := s.DB.Exec(`
		INSERT OR REPLACE INTO validated_items 
		(item_type, person_id, related_person_id, conflict_type, validation_note, reviewed_by, validated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, itemType, personID, relID, conflictType, note, reviewedBy)

	return err
}

// UnmarkAsReviewed removes a validated item marking.
func (s *Store) UnmarkAsReviewed(itemType string, personID int64, relatedPersonID *int64, conflictType string) error {
	var relID sql.NullInt64
	if relatedPersonID != nil {
		relID.Int64 = *relatedPersonID
		relID.Valid = true
	}

	_, err := s.DB.Exec(`
		DELETE FROM validated_items 
		WHERE item_type = ? AND person_id = ? AND 
		      (related_person_id IS ? OR (related_person_id IS NULL AND ? IS NULL)) AND 
		      conflict_type = ?
	`, itemType, personID, relID, relID, conflictType)

	return err
}

// IsItemReviewed checks if a specific item has been reviewed.
func (s *Store) IsItemReviewed(itemType string, personID int64, relatedPersonID *int64, conflictType string) (bool, *ValidatedItem, error) {
	var relID sql.NullInt64
	if relatedPersonID != nil {
		relID.Int64 = *relatedPersonID
		relID.Valid = true
	}

	row := s.DB.QueryRow(`
		SELECT id, item_type, person_id, related_person_id, conflict_type, validation_note, reviewed_by, validated_at
		FROM validated_items
		WHERE item_type = ? AND person_id = ? AND 
		      ((related_person_id = ? AND ? IS NOT NULL) OR (related_person_id IS NULL AND ? IS NULL)) AND 
		      conflict_type = ?
	`, itemType, personID, relID, relID, relID, conflictType)

	var v ValidatedItem
	var validatedAt, note, reviewedBy sql.NullString
	var relatedID sql.NullInt64

	err := row.Scan(&v.ID, &v.ItemType, &v.PersonID, &relatedID, &v.ConflictType, &note, &reviewedBy, &validatedAt)
	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	if relatedID.Valid {
		v.RelatedPersonID = &relatedID.Int64
	}
	if note.Valid {
		v.ValidationNote = note.String
	}
	if reviewedBy.Valid {
		v.ReviewedBy = reviewedBy.String
	}
	if validatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, validatedAt.String); err == nil {
			v.ValidatedAt = t
		}
	}

	return true, &v, nil
}

// GetAllValidatedItems returns all reviewed items.
func (s *Store) GetAllValidatedItems() ([]ValidatedItem, error) {
	rows, err := s.DB.Query(`
		SELECT id, item_type, person_id, related_person_id, conflict_type, validation_note, reviewed_by, validated_at
		FROM validated_items
		ORDER BY validated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ValidatedItem
	for rows.Next() {
		var v ValidatedItem
		var validatedAt, note, reviewedBy sql.NullString
		var relatedID sql.NullInt64

		if err := rows.Scan(&v.ID, &v.ItemType, &v.PersonID, &relatedID, &v.ConflictType, &note, &reviewedBy, &validatedAt); err != nil {
			return nil, err
		}

		if relatedID.Valid {
			v.RelatedPersonID = &relatedID.Int64
		}
		if note.Valid {
			v.ValidationNote = note.String
		}
		if reviewedBy.Valid {
			v.ReviewedBy = reviewedBy.String
		}
		if validatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, validatedAt.String); err == nil {
				v.ValidatedAt = t
			}
		}

		items = append(items, v)
	}

	return items, rows.Err()
}

// ============================================================================
// Research Todo Methods
// ============================================================================

// CreateTodo creates a new research todo for a person.
func (s *Store) CreateTodo(todo *ResearchTodo) error {
	res, err := s.DB.Exec(`
		INSERT INTO research_todos (person_id, description, priority, status, notes)
		VALUES (?, ?, ?, ?, ?)
	`, todo.PersonID, todo.Description, todo.Priority, todo.Status, todo.Notes)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	todo.ID = id
	todo.CreatedAt = time.Now()
	todo.UpdatedAt = time.Now()
	return nil
}

// UpdateTodo updates an existing research todo.
func (s *Store) UpdateTodo(todo *ResearchTodo) error {
	_, err := s.DB.Exec(`
		UPDATE research_todos 
		SET description=?, priority=?, status=?, notes=?, completed_at=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, todo.Description, todo.Priority, todo.Status, todo.Notes, todo.CompletedAt, todo.ID)
	return err
}

// DeleteTodo deletes a research todo.
func (s *Store) DeleteTodo(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM research_todos WHERE id=?`, id)
	return err
}

// GetTodoByID gets a specific todo by ID.
func (s *Store) GetTodoByID(id int64) (*ResearchTodo, error) {
	var todo ResearchTodo
	var notes sql.NullString
	var completedAt sql.NullString
	var createdAt sql.NullString
	var updatedAt sql.NullString

	row := s.DB.QueryRow(`
		SELECT id, person_id, description, priority, status, notes, created_at, completed_at, updated_at
		FROM research_todos
		WHERE id=?
	`, id)

	err := row.Scan(&todo.ID, &todo.PersonID, &todo.Description, &todo.Priority, &todo.Status, &notes, &createdAt, &completedAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if notes.Valid {
		todo.Notes = notes.String
	}
	if completedAt.Valid {
		if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
			todo.CompletedAt = &t
		}
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			todo.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			todo.UpdatedAt = t
		}
	}

	return &todo, nil
}

// GetTodosForPerson gets all todos for a specific person.
func (s *Store) GetTodosForPerson(personID int64) ([]ResearchTodo, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, description, priority, status, notes, created_at, completed_at, updated_at
		FROM research_todos
		WHERE person_id=?
		ORDER BY 
			CASE status 
				WHEN 'pending' THEN 1 
				WHEN 'completed' THEN 2 
			END,
			CASE priority 
				WHEN 'high' THEN 1 
				WHEN 'medium' THEN 2 
				WHEN 'low' THEN 3 
			END,
			created_at DESC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []ResearchTodo
	for rows.Next() {
		var todo ResearchTodo
		var notes sql.NullString
		var completedAt sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString

		if err := rows.Scan(&todo.ID, &todo.PersonID, &todo.Description, &todo.Priority, &todo.Status, &notes, &createdAt, &completedAt, &updatedAt); err != nil {
			return nil, err
		}

		if notes.Valid {
			todo.Notes = notes.String
		}
		if completedAt.Valid {
			if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				todo.CompletedAt = &t
			}
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				todo.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				todo.UpdatedAt = t
			}
		}

		todos = append(todos, todo)
	}

	return todos, rows.Err()
}

// GetAllPendingTodos gets all pending todos across all people.
func (s *Store) GetAllPendingTodos() ([]ResearchTodo, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, description, priority, status, notes, created_at, completed_at, updated_at
		FROM research_todos
		WHERE status='pending'
		ORDER BY 
			CASE priority 
				WHEN 'high' THEN 1 
				WHEN 'medium' THEN 2 
				WHEN 'low' THEN 3 
			END,
			created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []ResearchTodo
	for rows.Next() {
		var todo ResearchTodo
		var notes sql.NullString
		var completedAt sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString

		if err := rows.Scan(&todo.ID, &todo.PersonID, &todo.Description, &todo.Priority, &todo.Status, &notes, &createdAt, &completedAt, &updatedAt); err != nil {
			return nil, err
		}

		if notes.Valid {
			todo.Notes = notes.String
		}
		if completedAt.Valid {
			if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				todo.CompletedAt = &t
			}
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				todo.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				todo.UpdatedAt = t
			}
		}

		todos = append(todos, todo)
	}

	return todos, rows.Err()
}

// GetAllCompletedTodos gets all completed todos across all people, ordered by person.
func (s *Store) GetAllCompletedTodos() ([]ResearchTodo, error) {
	rows, err := s.DB.Query(`
		SELECT rt.id, rt.person_id, rt.description, rt.priority, rt.status, rt.notes, rt.created_at, rt.completed_at, rt.updated_at
		FROM research_todos rt
		INNER JOIN persons p ON rt.person_id = p.id
		WHERE rt.status='completed'
		ORDER BY p.surname, p.given_name, rt.completed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []ResearchTodo
	for rows.Next() {
		var todo ResearchTodo
		var notes sql.NullString
		var completedAt sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString

		if err := rows.Scan(&todo.ID, &todo.PersonID, &todo.Description, &todo.Priority, &todo.Status, &notes, &createdAt, &completedAt, &updatedAt); err != nil {
			return nil, err
		}

		if notes.Valid {
			todo.Notes = notes.String
		}
		if completedAt.Valid {
			if t, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				todo.CompletedAt = &t
			}
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				todo.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				todo.UpdatedAt = t
			}
		}

		todos = append(todos, todo)
	}

	return todos, rows.Err()
}

// CountPendingTodosForPerson returns the count of pending todos for a person.
func (s *Store) CountPendingTodosForPerson(personID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*) FROM research_todos WHERE person_id=? AND status='pending'
	`, personID).Scan(&count)
	return count, err
}

// MarkTodoComplete marks a todo as completed.
func (s *Store) MarkTodoComplete(todoID int64) error {
	now := time.Now()
	_, err := s.DB.Exec(`
		UPDATE research_todos 
		SET status='completed', completed_at=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, now, todoID)
	return err
}

// MarkTodoPending marks a todo as pending (uncomplete).
func (s *Store) MarkTodoPending(todoID int64) error {
	_, err := s.DB.Exec(`
		UPDATE research_todos 
		SET status='pending', completed_at=NULL, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, todoID)
	return err
}

// ============================================================================
// Source and Citation Methods
// ============================================================================

// CreateSource creates a new source.
func (s *Store) CreateSource(source *Source) error {
	res, err := s.DB.Exec(`
		INSERT INTO sources (title, author, publication, repository, call_number, notes, source_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, source.Title, source.Author, source.Publication, source.Repository, source.CallNumber, source.Notes, source.SourceType)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	source.ID = id
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	return nil
}

// UpdateSource updates an existing source.
func (s *Store) UpdateSource(source *Source) error {
	_, err := s.DB.Exec(`
		UPDATE sources 
		SET title=?, author=?, publication=?, repository=?, call_number=?, notes=?, source_type=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, source.Title, source.Author, source.Publication, source.Repository, source.CallNumber, source.Notes, source.SourceType, source.ID)
	return err
}

// DeleteSource deletes a source and all its citations.
func (s *Store) DeleteSource(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM sources WHERE id=?`, id)
	return err
}

// GetSourceByID gets a specific source by ID.
func (s *Store) GetSourceByID(id int64) (*Source, error) {
	var source Source
	var author, publication, repository, callNumber, notes, sourceType sql.NullString
	var createdAt, updatedAt sql.NullString

	row := s.DB.QueryRow(`
		SELECT id, title, author, publication, repository, call_number, notes, source_type, created_at, updated_at
		FROM sources
		WHERE id=?
	`, id)

	err := row.Scan(&source.ID, &source.Title, &author, &publication, &repository, &callNumber, &notes, &sourceType, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if author.Valid {
		source.Author = author.String
	}
	if publication.Valid {
		source.Publication = publication.String
	}
	if repository.Valid {
		source.Repository = repository.String
	}
	if callNumber.Valid {
		source.CallNumber = callNumber.String
	}
	if notes.Valid {
		source.Notes = notes.String
	}
	if sourceType.Valid {
		source.SourceType = sourceType.String
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			source.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			source.UpdatedAt = t
		}
	}

	return &source, nil
}

// GetAllSources returns all sources.
func (s *Store) GetAllSources() ([]Source, error) {
	rows, err := s.DB.Query(`
		SELECT id, title, author, publication, repository, call_number, notes, source_type, created_at, updated_at
		FROM sources
		ORDER BY title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var source Source
		var author, publication, repository, callNumber, notes, sourceType sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&source.ID, &source.Title, &author, &publication, &repository, &callNumber, &notes, &sourceType, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		if author.Valid {
			source.Author = author.String
		}
		if publication.Valid {
			source.Publication = publication.String
		}
		if repository.Valid {
			source.Repository = repository.String
		}
		if callNumber.Valid {
			source.CallNumber = callNumber.String
		}
		if notes.Valid {
			source.Notes = notes.String
		}
		if sourceType.Valid {
			source.SourceType = sourceType.String
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				source.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				source.UpdatedAt = t
			}
		}

		sources = append(sources, source)
	}

	return sources, rows.Err()
}

// CreateCitation creates a new citation linking a source to a person.
func (s *Store) CreateCitation(citation *Citation) error {
	res, err := s.DB.Exec(`
		INSERT INTO citations (source_id, person_id, citation_detail, transcription, confidence, notes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, citation.SourceID, citation.PersonID, citation.CitationDetail, citation.Transcription, citation.Confidence, citation.Notes)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	citation.ID = id
	citation.CreatedAt = time.Now()
	citation.UpdatedAt = time.Now()
	return nil
}

// UpdateCitation updates an existing citation.
func (s *Store) UpdateCitation(citation *Citation) error {
	_, err := s.DB.Exec(`
		UPDATE citations 
		SET citation_detail=?, transcription=?, confidence=?, notes=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, citation.CitationDetail, citation.Transcription, citation.Confidence, citation.Notes, citation.ID)
	return err
}

// DeleteCitation deletes a citation.
func (s *Store) DeleteCitation(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM citations WHERE id=?`, id)
	return err
}

// GetCitationsForPerson gets all citations for a specific person.
func (s *Store) GetCitationsForPerson(personID int64) ([]Citation, error) {
	rows, err := s.DB.Query(`
		SELECT id, source_id, person_id, citation_detail, transcription, confidence, notes, created_at, updated_at
		FROM citations
		WHERE person_id=?
		ORDER BY created_at DESC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var citations []Citation
	for rows.Next() {
		var citation Citation
		var citationDetail, transcription, confidence, notes sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&citation.ID, &citation.SourceID, &citation.PersonID, &citationDetail, &transcription, &confidence, &notes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		if citationDetail.Valid {
			citation.CitationDetail = citationDetail.String
		}
		if transcription.Valid {
			citation.Transcription = transcription.String
		}
		if confidence.Valid {
			citation.Confidence = confidence.String
		}
		if notes.Valid {
			citation.Notes = notes.String
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				citation.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				citation.UpdatedAt = t
			}
		}

		citations = append(citations, citation)
	}

	return citations, rows.Err()
}

// GetCitationsForSource gets all citations for a specific source.
func (s *Store) GetCitationsForSource(sourceID int64) ([]Citation, error) {
	rows, err := s.DB.Query(`
		SELECT id, source_id, person_id, citation_detail, transcription, confidence, notes, created_at, updated_at
		FROM citations
		WHERE source_id=?
		ORDER BY created_at DESC
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var citations []Citation
	for rows.Next() {
		var citation Citation
		var citationDetail, transcription, confidence, notes sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&citation.ID, &citation.SourceID, &citation.PersonID, &citationDetail, &transcription, &confidence, &notes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		if citationDetail.Valid {
			citation.CitationDetail = citationDetail.String
		}
		if transcription.Valid {
			citation.Transcription = transcription.String
		}
		if confidence.Valid {
			citation.Confidence = confidence.String
		}
		if notes.Valid {
			citation.Notes = notes.String
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				citation.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				citation.UpdatedAt = t
			}
		}

		citations = append(citations, citation)
	}

	return citations, rows.Err()
}

// CountCitationsForPerson returns the count of citations for a person.
func (s *Store) CountCitationsForPerson(personID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*) FROM citations WHERE person_id=?
	`, personID).Scan(&count)
	return count, err
}

// ========================================
// Research Log CRUD
// ========================================

// CountResearchLogsForPerson returns the number of research log entries linked to a person.
func (s *Store) CountResearchLogsForPerson(personID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*) FROM research_log_people WHERE person_id=?
	`, personID).Scan(&count)
	return count, err
}

// PersonIndicators holds precomputed presence flags used to render list/table
// row badges (todos, sources, research log, media) without per-row queries.
type PersonIndicators struct {
	HasPendingTodo bool
	HasCitation    bool
	HasResearchLog bool
	HasMedia       bool
}

// GetPersonIndicators returns indicator flags for every person that has at
// least one of: a pending todo, a citation, a research log entry, or linked
// media. It runs 4 aggregate queries total, regardless of how many people
// are in the database, instead of one query per person per indicator.
func (s *Store) GetPersonIndicators() (map[int64]PersonIndicators, error) {
	result := make(map[int64]PersonIndicators)

	mark := func(query string, apply func(*PersonIndicators)) error {
		rows, err := s.DB.Query(query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var personID int64
			if err := rows.Scan(&personID); err != nil {
				return err
			}
			ind := result[personID]
			apply(&ind)
			result[personID] = ind
		}
		return rows.Err()
	}

	if err := mark(`SELECT DISTINCT person_id FROM research_todos WHERE status='pending'`,
		func(ind *PersonIndicators) { ind.HasPendingTodo = true }); err != nil {
		return nil, err
	}
	if err := mark(`SELECT DISTINCT person_id FROM citations`,
		func(ind *PersonIndicators) { ind.HasCitation = true }); err != nil {
		return nil, err
	}
	if err := mark(`SELECT DISTINCT person_id FROM research_log_people`,
		func(ind *PersonIndicators) { ind.HasResearchLog = true }); err != nil {
		return nil, err
	}
	if err := mark(`SELECT DISTINCT person_id FROM person_media`,
		func(ind *PersonIndicators) { ind.HasMedia = true }); err != nil {
		return nil, err
	}

	return result, nil
}

// CreateResearchLog creates a new research log entry.
func (s *Store) CreateResearchLog(log *ResearchLog) error {
	res, err := s.DB.Exec(`
		INSERT INTO research_logs (person_id, search_date, repository, record_type, search_goal, results, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.PersonID, log.SearchDate, log.Repository, log.RecordType, log.SearchGoal, log.Results, log.Notes, time.Now(), time.Now())
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

// GetResearchLogByID retrieves a research log entry by ID.
func (s *Store) GetResearchLogByID(id int64) (*ResearchLog, error) {
	log := &ResearchLog{}
	var personID sql.NullInt64
	err := s.DB.QueryRow(`
		SELECT id, person_id, search_date, repository, record_type, search_goal, results, notes, created_at, updated_at
		FROM research_logs WHERE id=?
	`, id).Scan(&log.ID, &personID, &log.SearchDate, &log.Repository, &log.RecordType, &log.SearchGoal, &log.Results, &log.Notes, &log.CreatedAt, &log.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if personID.Valid {
		log.PersonID = &personID.Int64
	}
	return log, nil
}

// UpdateResearchLog updates an existing research log entry.
func (s *Store) UpdateResearchLog(log *ResearchLog) error {
	_, err := s.DB.Exec(`
		UPDATE research_logs SET person_id=?, search_date=?, repository=?, record_type=?, search_goal=?, results=?, notes=?, updated_at=?
		WHERE id=?
	`, log.PersonID, log.SearchDate, log.Repository, log.RecordType, log.SearchGoal, log.Results, log.Notes, time.Now(), log.ID)
	return err
}

// DeleteResearchLog deletes a research log entry.
func (s *Store) DeleteResearchLog(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM research_logs WHERE id=?`, id)
	return err
}

// GetAllResearchLogs retrieves all research log entries, ordered by search date (newest first).
func (s *Store) GetAllResearchLogs() ([]ResearchLog, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, search_date, repository, record_type, search_goal, results, notes, created_at, updated_at
		FROM research_logs
		ORDER BY search_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ResearchLog
	for rows.Next() {
		var log ResearchLog
		var personID sql.NullInt64
		if err := rows.Scan(&log.ID, &personID, &log.SearchDate, &log.Repository, &log.RecordType, &log.SearchGoal, &log.Results, &log.Notes, &log.CreatedAt, &log.UpdatedAt); err != nil {
			return nil, err
		}
		if personID.Valid {
			log.PersonID = &personID.Int64
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// GetResearchLogsForPerson retrieves all research log entries for a specific person.
func (s *Store) GetResearchLogsForPerson(personID int64) ([]ResearchLog, error) {
	rows, err := s.DB.Query(`
		SELECT DISTINCT rl.id, rl.person_id, rl.search_date, rl.repository, rl.record_type, rl.search_goal, rl.results, rl.notes, rl.created_at, rl.updated_at
		FROM research_logs rl
		INNER JOIN research_log_people rlp ON rl.id = rlp.research_log_id
		WHERE rlp.person_id=?
		ORDER BY rl.search_date DESC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ResearchLog
	for rows.Next() {
		var log ResearchLog
		var personID sql.NullInt64
		if err := rows.Scan(&log.ID, &personID, &log.SearchDate, &log.Repository, &log.RecordType, &log.SearchGoal, &log.Results, &log.Notes, &log.CreatedAt, &log.UpdatedAt); err != nil {
			return nil, err
		}
		if personID.Valid {
			log.PersonID = &personID.Int64
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// GetResearchLogsByRepository retrieves all research log entries for a specific repository.
func (s *Store) GetResearchLogsByRepository(repository string) ([]ResearchLog, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, search_date, repository, record_type, search_goal, results, notes, created_at, updated_at
		FROM research_logs
		WHERE repository=?
		ORDER BY search_date DESC
	`, repository)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ResearchLog
	for rows.Next() {
		var log ResearchLog
		var personID sql.NullInt64
		if err := rows.Scan(&log.ID, &personID, &log.SearchDate, &log.Repository, &log.RecordType, &log.SearchGoal, &log.Results, &log.Notes, &log.CreatedAt, &log.UpdatedAt); err != nil {
			return nil, err
		}
		if personID.Valid {
			log.PersonID = &personID.Int64
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// GetRecentResearchLogs retrieves the most recent N research log entries.
func (s *Store) GetRecentResearchLogs(limit int) ([]ResearchLog, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, search_date, repository, record_type, search_goal, results, notes, created_at, updated_at
		FROM research_logs
		ORDER BY search_date DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ResearchLog
	for rows.Next() {
		var log ResearchLog
		var personID sql.NullInt64
		if err := rows.Scan(&log.ID, &personID, &log.SearchDate, &log.Repository, &log.RecordType, &log.SearchGoal, &log.Results, &log.Notes, &log.CreatedAt, &log.UpdatedAt); err != nil {
			return nil, err
		}
		if personID.Valid {
			log.PersonID = &personID.Int64
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// LinkResearchLogToPerson links a research log entry to a person.
func (s *Store) LinkResearchLogToPerson(logID int64, personID int64) error {
	_, err := s.DB.Exec(`
		INSERT OR IGNORE INTO research_log_people (research_log_id, person_id)
		VALUES (?, ?)
	`, logID, personID)
	return err
}

// UnlinkResearchLogFromPerson unlinks a research log entry from a person.
func (s *Store) UnlinkResearchLogFromPerson(logID int64, personID int64) error {
	_, err := s.DB.Exec(`
		DELETE FROM research_log_people
		WHERE research_log_id=? AND person_id=?
	`, logID, personID)
	return err
}

// GetPeopleForResearchLog retrieves all people linked to a research log entry.
func (s *Store) GetPeopleForResearchLog(logID int64) ([]Person, error) {
	rows, err := s.DB.Query(`
		SELECT p.id, p.given_name, p.surname, p.preferred_name, p.gender, p.birth_date, p.birth_place, 
		       p.death_date, p.death_place, p.is_living, p.address, p.city, p.state, 
		       p.postal_code, p.country, p.email, p.phone, p.uid, p.notes, 
		       p.bookmarked, p.last_accessed, p.created_at, p.updated_at,
		       p.birth_year, p.death_year
		FROM persons p
		INNER JOIN research_log_people rlp ON p.id = rlp.person_id
		WHERE rlp.research_log_id=?
		ORDER BY p.surname, p.given_name
	`, logID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		var lastAccessed sql.NullTime
		var preferredName sql.NullString
		var birthYear, deathYear sql.NullInt64
		var birthDate, birthPlace, deathDate, deathPlace sql.NullString
		var isLiving sql.NullInt64
		var address, city, state, postalCode, country, email, phone, uid, notes sql.NullString
		var bookmarked sql.NullInt64

		if err := rows.Scan(&p.ID, &p.GivenName, &p.Surname, &preferredName, &p.Gender, &birthDate, &birthPlace,
			&deathDate, &deathPlace, &isLiving, &address, &city, &state,
			&postalCode, &country, &email, &phone, &uid, &notes,
			&bookmarked, &lastAccessed, &p.CreatedAt, &p.UpdatedAt,
			&birthYear, &deathYear); err != nil {
			return nil, err
		}

		if preferredName.Valid {
			p.PreferredName = preferredName.String
		}

		// Convert nullable fields
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
		if notes.Valid {
			p.Notes = notes.String
		}
		if bookmarked.Valid {
			p.Bookmarked = boolFromInt(int(bookmarked.Int64))
		}
		if lastAccessed.Valid {
			p.LastAccessed = &lastAccessed.Time
		}
		// birthYear and deathYear are legacy fields, we ignore them
		people = append(people, p)
	}
	return people, rows.Err()
}

// CreateSavedSearch creates a new saved search
func (s *Store) CreateSavedSearch(name string, criteriaJSON string) error {
	_, err := s.DB.Exec(`
		INSERT INTO saved_searches (name, criteria_json, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, name, criteriaJSON)
	return err
}

// GetSavedSearches returns all saved searches
func (s *Store) GetSavedSearches() ([]SavedSearch, error) {
	rows, err := s.DB.Query(`
		SELECT id, name, criteria_json, created_at, updated_at
		FROM saved_searches
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var searches []SavedSearch
	for rows.Next() {
		var ss SavedSearch
		if err := rows.Scan(&ss.ID, &ss.Name, &ss.CriteriaJSON, &ss.CreatedAt, &ss.UpdatedAt); err != nil {
			return nil, err
		}
		searches = append(searches, ss)
	}
	return searches, rows.Err()
}

// GetSavedSearchByID returns a saved search by ID
func (s *Store) GetSavedSearchByID(id int64) (*SavedSearch, error) {
	var ss SavedSearch
	err := s.DB.QueryRow(`
		SELECT id, name, criteria_json, created_at, updated_at
		FROM saved_searches
		WHERE id = ?
	`, id).Scan(&ss.ID, &ss.Name, &ss.CriteriaJSON, &ss.CreatedAt, &ss.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

// UpdateSavedSearch updates an existing saved search
func (s *Store) UpdateSavedSearch(id int64, name string, criteriaJSON string) error {
	_, err := s.DB.Exec(`
		UPDATE saved_searches
		SET name = ?, criteria_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, criteriaJSON, id)
	return err
}

// DeleteSavedSearch deletes a saved search
func (s *Store) DeleteSavedSearch(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM saved_searches WHERE id = ?`, id)
	return err
}

// GetProjectNotes retrieves the project-wide notes
func (s *Store) GetProjectNotes() (string, error) {
	var notes string
	err := s.DB.QueryRow(`SELECT notes FROM project_notes WHERE id = 1`).Scan(&notes)
	if err != nil {
		return "", err
	}
	return notes, nil
}

// SaveProjectNotes saves the project-wide notes
func (s *Store) SaveProjectNotes(notes string) error {
	_, err := s.DB.Exec(`
		UPDATE project_notes 
		SET notes = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`, notes)
	return err
}

// CreateAlternateName creates a new alternate name for a person
func (s *Store) CreateAlternateName(alt *AlternateName) error {
	result, err := s.DB.Exec(`
		INSERT INTO alternate_names (person_id, name_type, given_name, surname, notes)
		VALUES (?, ?, ?, ?, ?)
	`, alt.PersonID, alt.NameType, alt.GivenName, alt.Surname, alt.Notes)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	alt.ID = id
	return nil
}

// GetAlternateNames returns all alternate names for a person
func (s *Store) GetAlternateNames(personID int64) ([]AlternateName, error) {
	rows, err := s.DB.Query(`
		SELECT id, person_id, name_type, given_name, surname, notes, created_at
		FROM alternate_names
		WHERE person_id = ?
		ORDER BY created_at ASC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []AlternateName
	for rows.Next() {
		var alt AlternateName
		if err := rows.Scan(&alt.ID, &alt.PersonID, &alt.NameType, &alt.GivenName, &alt.Surname, &alt.Notes, &alt.CreatedAt); err != nil {
			return nil, err
		}
		names = append(names, alt)
	}
	return names, rows.Err()
}

// UpdateAlternateName updates an alternate name
func (s *Store) UpdateAlternateName(alt *AlternateName) error {
	_, err := s.DB.Exec(`
		UPDATE alternate_names
		SET name_type = ?, given_name = ?, surname = ?, notes = ?
		WHERE id = ?
	`, alt.NameType, alt.GivenName, alt.Surname, alt.Notes, alt.ID)
	return err
}

// DeleteAlternateName deletes an alternate name
func (s *Store) DeleteAlternateName(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM alternate_names WHERE id = ?`, id)
	return err
}

// CreateAlternatePlace creates a new alternate place name mapping
func (s *Store) CreateAlternatePlace(alt *AlternatePlace) error {
	result, err := s.DB.Exec(`
		INSERT INTO alternate_places (historical_name, current_name, year_changed, notes)
		VALUES (?, ?, ?, ?)
	`, alt.HistoricalName, alt.CurrentName, alt.YearChanged, alt.Notes)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	alt.ID = id
	return nil
}

// GetAllAlternatePlaces returns all alternate place name mappings
func (s *Store) GetAllAlternatePlaces() ([]AlternatePlace, error) {
	rows, err := s.DB.Query(`
		SELECT id, historical_name, current_name, year_changed, notes, created_at
		FROM alternate_places
		ORDER BY historical_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var places []AlternatePlace
	for rows.Next() {
		var alt AlternatePlace
		var yearChanged sql.NullInt64
		if err := rows.Scan(&alt.ID, &alt.HistoricalName, &alt.CurrentName, &yearChanged, &alt.Notes, &alt.CreatedAt); err != nil {
			return nil, err
		}
		if yearChanged.Valid {
			alt.YearChanged = int(yearChanged.Int64)
		}
		places = append(places, alt)
	}
	return places, rows.Err()
}

// GetAlternatePlaceFor returns the current name for a historical place name (if it exists)
func (s *Store) GetAlternatePlaceFor(historicalName string) (*AlternatePlace, error) {
	var alt AlternatePlace
	var yearChanged sql.NullInt64
	err := s.DB.QueryRow(`
		SELECT id, historical_name, current_name, year_changed, notes, created_at
		FROM alternate_places
		WHERE LOWER(historical_name) = LOWER(?)
		LIMIT 1
	`, historicalName).Scan(&alt.ID, &alt.HistoricalName, &alt.CurrentName, &yearChanged, &alt.Notes, &alt.CreatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil // No alternate found, not an error
	}
	if err != nil {
		return nil, err
	}
	
	if yearChanged.Valid {
		alt.YearChanged = int(yearChanged.Int64)
	}
	return &alt, nil
}

// UpdateAlternatePlace updates an alternate place name mapping
func (s *Store) UpdateAlternatePlace(alt *AlternatePlace) error {
	_, err := s.DB.Exec(`
		UPDATE alternate_places
		SET historical_name = ?, current_name = ?, year_changed = ?, notes = ?
		WHERE id = ?
	`, alt.HistoricalName, alt.CurrentName, alt.YearChanged, alt.Notes, alt.ID)
	return err
}

// DeleteAlternatePlace deletes an alternate place name mapping
func (s *Store) DeleteAlternatePlace(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM alternate_places WHERE id = ?`, id)
	return err
}

// GetLastBackupDate retrieves the last backup date
func (s *Store) GetLastBackupDate() (*time.Time, error) {
	var dateStr sql.NullString
	err := s.DB.QueryRow(`SELECT last_backup_date FROM settings WHERE id = 1`).Scan(&dateStr)
	if err != nil {
		return nil, err
	}
	if !dateStr.Valid || dateStr.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, dateStr.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateLastBackupDate updates the last backup timestamp
func (s *Store) UpdateLastBackupDate() error {
	_, err := s.DB.Exec(`
		UPDATE settings 
		SET last_backup_date = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`, time.Now().Format(time.RFC3339))
	return err
}

// GetBackupReminderEnabled checks if backup reminders are enabled
func (s *Store) GetBackupReminderEnabled() (bool, error) {
	var enabled int
	err := s.DB.QueryRow(`SELECT backup_reminder_enabled FROM settings WHERE id = 1`).Scan(&enabled)
	if err != nil {
		return true, err // default to enabled
	}
	return enabled == 1, nil
}

// GetBackupReminderSettings retrieves all backup reminder settings in one query (optimized)
func (s *Store) GetBackupReminderSettings() (enabled bool, lastBackup *time.Time, days int, err error) {
	var enabledInt int
	var dateStr sql.NullString

	err = s.DB.QueryRow(`
		SELECT backup_reminder_enabled, last_backup_date, backup_reminder_days 
		FROM settings WHERE id = 1
	`).Scan(&enabledInt, &dateStr, &days)

	if err != nil {
		return true, nil, 7, err // defaults
	}

	enabled = enabledInt == 1

	if dateStr.Valid && dateStr.String != "" {
		t, parseErr := time.Parse(time.RFC3339, dateStr.String)
		if parseErr == nil {
			lastBackup = &t
		}
	}

	return enabled, lastBackup, days, nil
}

// SetBackupReminderEnabled enables/disables backup reminders
func (s *Store) SetBackupReminderEnabled(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.DB.Exec(`
		UPDATE settings 
		SET backup_reminder_enabled = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`, val)
	return err
}

// GetBackupReminderDays gets the number of days before reminding
func (s *Store) GetBackupReminderDays() (int, error) {
	var days int
	err := s.DB.QueryRow(`SELECT backup_reminder_days FROM settings WHERE id = 1`).Scan(&days)
	if err != nil {
		return 7, err // default to 7 days
	}
	return days, nil
}

// SetBackupReminderDays sets the number of days before reminding
func (s *Store) SetBackupReminderDays(days int) error {
	_, err := s.DB.Exec(`
		UPDATE settings 
		SET backup_reminder_days = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`, days)
	return err
}

// RebuildFullTextIndex rebuilds the FTS5 index with all current person data.
func (s *Store) RebuildFullTextIndex() error {
	// Clear existing FTS data
	_, err := s.DB.Exec(`DELETE FROM persons_fts;`)
	if err != nil {
		return err
	}
	
	// Populate FTS table from persons table with relationship data
	// Aggregate marriage places and spouse names for each person
	_, err = s.DB.Exec(`
		INSERT INTO persons_fts (
			person_id, given_name, surname, preferred_name, notes,
			birth_place, death_place, address, city, state, country, uid,
			marriage_places, spouse_names
		)
		SELECT 
			p.id, 
			p.given_name, 
			p.surname, 
			p.preferred_name, 
			p.notes,
			p.birth_place, 
			p.death_place, 
			p.address, 
			p.city, 
			p.state, 
			p.country, 
			p.uid,
			COALESCE(
				(SELECT GROUP_CONCAT(r.marriage_place, ' ')
				 FROM (SELECT DISTINCT marriage_place FROM relationships 
				       WHERE subject_id = p.id AND marriage_place IS NOT NULL AND marriage_place != '') r),
				''
			) as marriage_places,
			COALESCE(
				(SELECT GROUP_CONCAT(spouse_name, ' ')
				 FROM (SELECT DISTINCT (spouse.given_name || ' ' || spouse.surname) as spouse_name
				       FROM relationships r
				       JOIN persons spouse ON spouse.id = r.object_id
				       WHERE r.subject_id = p.id AND r.type IN ('spouse', 'partner')) s),
				''
			) as spouse_names
		FROM persons p;
	`)
	
	return err
}

// FullTextSearchResult represents a search result with context.
type FullTextSearchResult struct {
	PersonID     int64
	Person       Person
	MatchedField string // Which field matched (e.g., "notes", "birth_place")
	Snippet      string // Context snippet showing the match
	Rank         float64 // FTS5 relevance rank
}

// FullTextSearch performs a full-text search across all person data.
func (s *Store) FullTextSearch(query string, maxResults int) ([]FullTextSearchResult, error) {
	if query == "" {
		return []FullTextSearchResult{}, nil
	}
	
	if maxResults <= 0 {
		maxResults = 50 // Default limit
	}
	
	// Query FTS5 table with ranking
	rows, err := s.DB.Query(`
		SELECT 
			person_id,
			snippet(persons_fts, -1, '**', '**', '...', 30) as snippet,
			rank
		FROM persons_fts
		WHERE persons_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, maxResults)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []FullTextSearchResult
	for rows.Next() {
		var result FullTextSearchResult
		var snippet string
		var rank float64
		
		err := rows.Scan(&result.PersonID, &snippet, &rank)
		if err != nil {
			continue
		}
		
		// Get full person record
		person, err := s.GetPersonByID(result.PersonID)
		if err != nil {
			continue
		}
		
		result.Person = *person
		result.Snippet = snippet
		result.Rank = rank
		
		// Determine which field matched (look for ** markers in snippet)
		result.MatchedField = determineMatchedField(snippet, person)
		
		results = append(results, result)
	}
	
	return results, nil
}

// determineMatchedField identifies which field contains the search match.
func determineMatchedField(snippet string, person *Person) string {
	// The snippet contains the matched text with ** markers
	// Remove markers for comparison
	cleanSnippet := strings.ReplaceAll(snippet, "**", "")
	
	// Check which field the snippet came from
	// Priority order: more specific fields first
	if strings.Contains(person.GivenName, cleanSnippet) {
		return "Given Name"
	}
	if strings.Contains(person.Surname, cleanSnippet) {
		return "Surname"
	}
	if strings.Contains(person.PreferredName, cleanSnippet) {
		return "Preferred Name"
	}
	if strings.Contains(person.BirthPlace, cleanSnippet) {
		return "Birth Place"
	}
	if strings.Contains(person.DeathPlace, cleanSnippet) {
		return "Death Place"
	}
	if strings.Contains(person.Address, cleanSnippet) {
		return "Address"
	}
	if strings.Contains(person.City, cleanSnippet) {
		return "City"
	}
	if strings.Contains(person.State, cleanSnippet) {
		return "State"
	}
	if strings.Contains(person.Country, cleanSnippet) {
		return "Country"
	}
	if strings.Contains(person.Notes, cleanSnippet) {
		return "Notes"
	}
	if strings.Contains(person.UID, cleanSnippet) {
		return "UID"
	}
	
	// Check if it's from marriage places or spouse names
	// These require a database query since they're not in the Person struct
	if strings.Contains(strings.ToLower(snippet), "marriage") || 
	   strings.Contains(strings.ToLower(snippet), "married") {
		return "Marriage Place"
	}
	
	// Could be spouse name if it contains typical name patterns
	if len(strings.Fields(cleanSnippet)) >= 2 {
		return "Spouse Name"
	}
	
	return "Related Field"
}

// UpdateFullTextIndexForPerson updates the FTS index for a single person.
func (s *Store) UpdateFullTextIndexForPerson(personID int64) error {
	// Delete existing FTS entry
	_, err := s.DB.Exec(`DELETE FROM persons_fts WHERE person_id = ?`, personID)
	if err != nil {
		return err
	}
	
	// Re-insert from persons table with relationship data
	_, err = s.DB.Exec(`
		INSERT INTO persons_fts (
			person_id, given_name, surname, preferred_name, notes,
			birth_place, death_place, address, city, state, country, uid,
			marriage_places, spouse_names
		)
		SELECT 
			p.id, 
			p.given_name, 
			p.surname, 
			p.preferred_name, 
			p.notes,
			p.birth_place, 
			p.death_place, 
			p.address, 
			p.city, 
			p.state, 
			p.country, 
			p.uid,
			COALESCE(
				(SELECT GROUP_CONCAT(r.marriage_place, ' ')
				 FROM (SELECT DISTINCT marriage_place FROM relationships 
				       WHERE subject_id = p.id AND marriage_place IS NOT NULL AND marriage_place != '') r),
				''
			) as marriage_places,
			COALESCE(
				(SELECT GROUP_CONCAT(spouse_name, ' ')
				 FROM (SELECT DISTINCT (spouse.given_name || ' ' || spouse.surname) as spouse_name
				       FROM relationships r
				       JOIN persons spouse ON spouse.id = r.object_id
				       WHERE r.subject_id = p.id AND r.type IN ('spouse', 'partner')) s),
				''
			) as spouse_names
		FROM persons p
		WHERE p.id = ?;
	`, personID)
	
	return err
}

// DeleteFullTextIndexForPerson removes a person from the FTS index.
func (s *Store) DeleteFullTextIndexForPerson(personID int64) error {
	_, err := s.DB.Exec(`DELETE FROM persons_fts WHERE person_id = ?`, personID)
	return err
}
