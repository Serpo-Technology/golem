package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// MediaStore is the content-addressed folder behind perception: the bytes a
// transducer captioned, an image, an audio clip, a document, kept on disk under
// the hash of their own content. The hash is the name, so a fragment's media
// reference can never dangle and identical media is stored once. The brain
// never reasons over these bytes; it reasons over the caption the transducer
// produced from them. The reference is only the handle that lets a transducer
// look again, for a detail the first pass dropped or a better captioner later.
type MediaStore struct {
	dir string
}

// OpenMediaStore prepares the media folder under a home directory.
func OpenMediaStore(home string) (*MediaStore, error) {
	dir := filepath.Join(home, "media")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &MediaStore{dir: dir}, nil
}

// Put stores bytes under the hex of their sha256 and returns that hash, writing
// the file only when it is new so identical media dedupes. The write lands
// through a temp rename, so a kill mid-write cannot leave a torn blob under a
// hash that claims to be whole.
func (m *MediaStore) Put(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(m.dir, hash)
	if _, err := os.Stat(path); err == nil {
		return hash, nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return hash, nil
}

// Path is where a hash's bytes live, whether or not they are present.
func (m *MediaStore) Path(hash string) string {
	return filepath.Join(m.dir, hash)
}

// Has reports whether a hash's bytes are on disk.
func (m *MediaStore) Has(hash string) bool {
	if hash == "" {
		return false
	}
	_, err := os.Stat(m.Path(hash))
	return err == nil
}

// SetMedia records that an episode's perception kept media of a given content
// hash and mime type, so the raw can be re-examined later. It is a sparse side
// table on the fragment graph: only a perceptual moment carries one.
func (s *Store) SetMedia(fragmentID uint64, hash, mime string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO media (fragment_id, hash, mime) VALUES (?, ?, ?)`,
		fragmentID, hash, mime)
	return err
}

// Media reads an episode's media reference; ok is false when it kept none.
func (s *Store) Media(fragmentID uint64) (hash, mime string, ok bool) {
	row := s.db.QueryRow(`SELECT hash, mime FROM media WHERE fragment_id = ?`, fragmentID)
	if err := row.Scan(&hash, &mime); err != nil {
		return "", "", false
	}
	return hash, mime, true
}
