package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// MergePersonRecords folds the duplicate person deleteID into keepID in a
// single transaction: keepID is updated to merged, every record that belonged
// to deleteID (relationships, events, citations, media links, alternate names,
// to-dos, research logs, identifiers) moves to keepID, and deleteID is removed.
// Nothing changes if any step fails.
func (s *Store) MergePersonRecords(keepID, deleteID int64, merged Person) error {
	if keepID == deleteID {
		return fmt.Errorf("cannot merge a person into themselves")
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var dupGiven, dupSurname string
	var dupBookmarked bool
	if err := tx.QueryRow(`SELECT COALESCE(given_name,''), COALESCE(surname,''), COALESCE(bookmarked,0) FROM persons WHERE id=?`, deleteID).
		Scan(&dupGiven, &dupSurname, &dupBookmarked); err != nil {
		return fmt.Errorf("duplicate person %d not found: %w", deleteID, err)
	}
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM persons WHERE id=?`, keepID).Scan(&exists); err != nil || exists == 0 {
		return fmt.Errorf("person %d to keep not found", keepID)
	}

	// 1. Update the kept person with the merged field values
	merged.ID = keepID
	merged.Bookmarked = merged.Bookmarked || dupBookmarked
	if _, err := tx.Exec(`UPDATE persons SET given_name=?,surname=?,preferred_name=?,gender=?,birth_date=?,birth_place=?,death_date=?,death_place=?,is_living=?,address=?,city=?,state=?,postal_code=?,country=?,email=?,phone=?,uid=?,notes=?,bookmarked=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		merged.GivenName, merged.Surname, merged.PreferredName, merged.Gender, merged.BirthDate, merged.BirthPlace, merged.DeathDate, merged.DeathPlace, intFromBool(merged.IsLiving), merged.Address, merged.City, merged.State, merged.PostalCode, merged.Country, merged.Email, merged.Phone, merged.UID, merged.Notes, intFromBool(merged.Bookmarked), keepID); err != nil {
		return fmt.Errorf("update kept person: %w", err)
	}

	// 2. Re-point relationships, dropping self links and ones keepID already has
	type relRow struct {
		id, subject, object                      int64
		relType                                  string
		marriageDate, marriagePlace, divorceDate string
		separationDate, endReason                string
	}
	rows, err := tx.Query(`SELECT id,subject_id,object_id,type,COALESCE(marriage_date,''),COALESCE(marriage_place,''),COALESCE(divorce_date,''),COALESCE(separation_date,''),COALESCE(end_reason,'') FROM relationships WHERE subject_id=? OR object_id=?`, deleteID, deleteID)
	if err != nil {
		return err
	}
	var rels []relRow
	for rows.Next() {
		var r relRow
		if err := rows.Scan(&r.id, &r.subject, &r.object, &r.relType, &r.marriageDate, &r.marriagePlace, &r.divorceDate, &r.separationDate, &r.endReason); err != nil {
			rows.Close()
			return err
		}
		rels = append(rels, r)
	}
	rows.Close()

	affected := map[int64]bool{keepID: true}
	for _, r := range rels {
		ns, no := r.subject, r.object
		if ns == deleteID {
			ns = keepID
		}
		if no == deleteID {
			no = keepID
		}
		if ns == no {
			if _, err := tx.Exec(`DELETE FROM relationships WHERE id=?`, r.id); err != nil {
				return err
			}
			continue
		}
		affected[ns], affected[no] = true, true

		var dupID int64
		var cur relRow
		err := tx.QueryRow(`SELECT id,COALESCE(marriage_date,''),COALESCE(marriage_place,''),COALESCE(divorce_date,''),COALESCE(separation_date,''),COALESCE(end_reason,'') FROM relationships WHERE subject_id=? AND object_id=? AND type=? AND id<>?`,
			ns, no, r.relType, r.id).Scan(&dupID, &cur.marriageDate, &cur.marriagePlace, &cur.divorceDate, &cur.separationDate, &cur.endReason)
		switch {
		case err == nil:
			// keepID already has this link: keep it, fill in any blank details, drop the duplicate
			if _, err := tx.Exec(`UPDATE relationships SET marriage_date=?,marriage_place=?,divorce_date=?,separation_date=?,end_reason=? WHERE id=?`,
				firstNonEmpty(cur.marriageDate, r.marriageDate), firstNonEmpty(cur.marriagePlace, r.marriagePlace),
				firstNonEmpty(cur.divorceDate, r.divorceDate), firstNonEmpty(cur.separationDate, r.separationDate),
				firstNonEmpty(cur.endReason, r.endReason), dupID); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM relationships WHERE id=?`, r.id); err != nil {
				return err
			}
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec(`UPDATE relationships SET subject_id=?, object_id=? WHERE id=?`, ns, no, r.id); err != nil {
				return err
			}
		default:
			return err
		}
	}

	// 3. Move everything else owned by the duplicate
	for _, table := range []string{"events", "identifiers", "citations", "research_todos", "alternate_names", "research_logs"} {
		if _, err := tx.Exec(`UPDATE `+table+` SET person_id=? WHERE person_id=?`, keepID, deleteID); err != nil {
			return fmt.Errorf("move %s: %w", table, err)
		}
	}
	// These have a unique (person, item) pair, so skip links keepID already has
	for _, table := range []string{"person_media", "research_log_people"} {
		if _, err := tx.Exec(`UPDATE OR IGNORE `+table+` SET person_id=? WHERE person_id=?`, keepID, deleteID); err != nil {
			return fmt.Errorf("move %s: %w", table, err)
		}
	}
	// Reviews ("not a duplicate", reviewed conflicts) about the removed record no longer apply
	if _, err := tx.Exec(`DELETE FROM validated_items WHERE person_id=? OR related_person_id=?`, deleteID, deleteID); err != nil {
		return err
	}

	// 4. Remember the duplicate's name as an alternate name if it differs
	if strings.TrimSpace(dupGiven+dupSurname) != "" &&
		!(strings.EqualFold(dupGiven, merged.GivenName) && strings.EqualFold(dupSurname, merged.Surname)) {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM alternate_names WHERE person_id=? AND lower(COALESCE(given_name,''))=lower(?) AND lower(COALESCE(surname,''))=lower(?)`,
			keepID, dupGiven, dupSurname).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := tx.Exec(`INSERT INTO alternate_names(person_id,name_type,given_name,surname,notes) VALUES (?,?,?,?,?)`,
				keepID, "spelling", dupGiven, dupSurname, fmt.Sprintf("Name on merged duplicate record #%d", deleteID)); err != nil {
				return err
			}
		}
	}

	// 5. Remove the duplicate (any rows left pointing at it cascade away)
	if _, err := tx.Exec(`DELETE FROM persons WHERE id=?`, deleteID); err != nil {
		return fmt.Errorf("delete duplicate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	_ = s.DeleteFullTextIndexForPerson(deleteID)
	for id := range affected {
		_ = s.UpdateFullTextIndexForPerson(id)
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
