package models

import (
	"context"
	"database/sql"
	"time"
)

// Tag is a color-coded label that can be applied to multiple monitors.
type Tag struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Color string `db:"color"` // CSS hex color, e.g. "#6366f1"
}

// TagStore handles all tag-related DB operations.
type TagStore struct {
	db *sql.DB
}

// NewTagStore creates a new TagStore.
func NewTagStore(db *sql.DB) *TagStore {
	return &TagStore{db: db}
}

// List returns all tags ordered by name.
func (s *TagStore) List() ([]*Tag, error) {
	rows, err := s.db.Query(`SELECT id, name, color FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		t := &Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// Get returns a single tag by ID.
func (s *TagStore) Get(id int64) (*Tag, error) {
	t := &Tag{}
	err := s.db.QueryRow(`SELECT id, name, color FROM tags WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Color)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// Create inserts a new tag and returns its ID.
func (s *TagStore) Create(t *Tag) (int64, error) {
	res, err := s.db.ExecContext(context.Background(),
		`INSERT INTO tags (name, color) VALUES (?, ?)`, t.Name, t.Color)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing tag.
func (s *TagStore) Update(t *Tag) error {
	_, err := s.db.ExecContext(context.Background(),
		`UPDATE tags SET name=?, color=? WHERE id=?`, t.Name, t.Color, t.ID)
	return err
}

// Delete removes a tag and all its monitor associations.
func (s *TagStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM tags WHERE id = ?`, id)
	return err
}

// ListForMonitor returns all tags assigned to a specific monitor.
func (s *TagStore) ListForMonitor(monitorID int64) ([]*Tag, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.color FROM tags t
		JOIN monitor_tags mt ON mt.tag_id = t.id
		WHERE mt.monitor_id = ?
		ORDER BY t.name ASC
	`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		t := &Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// SetMonitorTags replaces all tag assignments for a monitor with the given set of tag IDs.
func (s *TagStore) SetMonitorTags(monitorID int64, tagIDs []int64) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM monitor_tags WHERE monitor_id = ?`, monitorID); err != nil {
		return err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO monitor_tags (monitor_id, tag_id) VALUES (?, ?)`, monitorID, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListByTag returns all monitor IDs that have a specific tag.
func (s *TagStore) ListMonitorIDsByTag(tagID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT monitor_id FROM monitor_tags WHERE tag_id = ?`, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TagMapForMonitors returns a map of monitorID → tags for the given monitor IDs.
// Used to batch-load tag data for the dashboard.
func (s *TagStore) TagMapForMonitors(monitorIDs []int64) (map[int64][]*Tag, error) {
	if len(monitorIDs) == 0 {
		return map[int64][]*Tag{}, nil
	}

	result := make(map[int64][]*Tag, len(monitorIDs))

	// Build per-monitor queries (SQLite has no efficient parameterized IN for slices).
	for _, mid := range monitorIDs {
		tags, err := s.ListForMonitor(mid)
		if err != nil {
			return nil, err
		}
		if tags == nil {
			tags = []*Tag{}
		}
		result[mid] = tags
	}
	return result, nil
}

// TagsUpdatedAt returns the last update timestamp across all tags (for cache busting).
func (s *TagStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n)
	return n, err
}

// Computed helper: returns the current time.
func now() time.Time { return time.Now().UTC() }
