package memory

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func fragByID(t *testing.T, s *Store, id uint64) *Fragment {
	t.Helper()
	fs, err := s.FragmentsByIDs([]uint64{id})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) == 0 {
		t.Fatalf("fragment %d not found", id)
	}
	return fs[0]
}

// A lived fragment carries no edit mark until it is touched; MarkEdited sets it,
// so a shaped memory can be told from a lived one.
func TestMarkEditedSetsProvenance(t *testing.T) {
	s, _ := tempStore(t)
	f := encodeAt(t, s, "tell", "a small grey owl", 4, time.Now())
	if fragByID(t, s, f.ID).Edited {
		t.Fatal("a freshly lived fragment should not be marked edited")
	}
	if err := s.MarkEdited(f.ID); err != nil {
		t.Fatal(err)
	}
	if !fragByID(t, s, f.ID).Edited {
		t.Fatal("MarkEdited should set the provenance mark")
	}
}

// UpdateContent splices the content and the template prediction runs on and
// marks the fragment edited in one write.
func TestUpdateContentSplicesAndMarks(t *testing.T) {
	s, _ := tempStore(t)
	f := encodeAt(t, s, "tell", "a small grey owl", 4, time.Now())
	if err := s.UpdateContent(f.ID, "a large grey owl", "a <size> grey owl"); err != nil {
		t.Fatal(err)
	}
	got := fragByID(t, s, f.ID)
	if got.Content != "a large grey owl" || got.Template != "a <size> grey owl" {
		t.Fatalf("content and template should be spliced: %+v", got)
	}
	if !got.Edited {
		t.Fatal("a raw content edit should mark the fragment edited")
	}
}

// DeleteByID takes exactly the node named and the edges touching it, leaving its
// neighbors standing, and reports nothing gone when the node is already absent.
func TestDeleteByIDTakesOneNode(t *testing.T) {
	s, _ := tempStore(t)
	a := encodeAt(t, s, "tell", "a brass key", 4, time.Now())
	b := encodeAt(t, s, "tell", "a green bottle", 4, time.Now())
	if err := s.Link(a.ID, b.ID, 1.0); err != nil {
		t.Fatal(err)
	}
	n, err := s.DeleteByID(a.ID)
	if err != nil || n != 1 {
		t.Fatalf("deleting one node should report one gone: n=%d err=%v", n, err)
	}
	if fs, _ := s.FragmentsByIDs([]uint64{a.ID}); len(fs) != 0 {
		t.Fatal("the deleted node should be gone")
	}
	if fs, _ := s.FragmentsByIDs([]uint64{b.ID}); len(fs) == 0 {
		t.Fatal("the neighbor should survive")
	}
	if edgeWeight(t, s, a.ID, b.ID) != 0 {
		t.Fatal("the edge touching the deleted node should be gone")
	}
	if n, _ := s.DeleteByID(a.ID); n != 0 {
		t.Fatalf("re-deleting should report nothing gone: %d", n)
	}
}

// A database written before the provenance mark gains the column on open and its
// fragments read back unmarked, so a deployed memory migrates in place.
func TestEditedColumnMigratesOldDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE fragments (
		id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL, content TEXT NOT NULL,
		template TEXT NOT NULL, surprise REAL NOT NULL, dopamine REAL NOT NULL,
		noradrenaline REAL NOT NULL, serotonin REAL NOT NULL, at INTEGER NOT NULL,
		weight REAL NOT NULL, count INTEGER NOT NULL DEFAULT 1,
		kind TEXT NOT NULL DEFAULT 'episodic', lang TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '');`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO fragments
		(channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight)
		VALUES ('tell','a lived moment','a lived moment',4,0.2,0.1,0.5,?,1.0)`,
		time.Now().UnixNano()); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("opening an older database should migrate it, not fail: %v", err)
	}
	defer s.Close()
	got := fragByID(t, s, 1)
	if got.Content != "a lived moment" || got.Edited {
		t.Fatalf("the migrated fragment should read back intact and unmarked: %+v", got)
	}
}
