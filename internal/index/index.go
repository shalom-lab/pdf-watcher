package index

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Record struct {
	ServerID      string
	FileHash      string
	Path          string
	DOI           string
	PMID          string
	PMCID         string
	TitleNorm     string
	FamilyKey     string
	ItemKey       string
	AttachmentKey string
	Source        string
	IngestedAt    int64
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS ingest_files (
  server_id TEXT NOT NULL,
  file_hash TEXT NOT NULL,
  path TEXT,
  doi TEXT,
  pmid TEXT,
  pmcid TEXT,
  title_norm TEXT,
  item_key TEXT NOT NULL,
  attachment_key TEXT,
  source TEXT,
  ingested_at INTEGER NOT NULL,
  PRIMARY KEY (server_id, file_hash)
);
CREATE INDEX IF NOT EXISTS idx_ingest_doi ON ingest_files(server_id, doi);
CREATE INDEX IF NOT EXISTS idx_ingest_pmid ON ingest_files(server_id, pmid);
CREATE INDEX IF NOT EXISTS idx_ingest_pmcid ON ingest_files(server_id, pmcid);
CREATE INDEX IF NOT EXISTS idx_ingest_title ON ingest_files(server_id, title_norm);
CREATE INDEX IF NOT EXISTS idx_ingest_item ON ingest_files(server_id, item_key);
`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("ingest_files", "family_key", "TEXT"); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ingest_family ON ingest_files(server_id, family_key)`)
	return err
}

func (s *Store) ensureColumn(table, col, typ string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		_ = ctype
		if strings.EqualFold(name, col) {
			return nil
		}
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + typ)
	return err
}

const selectCols = `server_id,file_hash,path,doi,pmid,pmcid,title_norm,IFNULL(family_key,''),item_key,attachment_key,source,ingested_at`

func (s *Store) FindByHash(serverID, hash string) (*Record, error) {
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND file_hash=?`, serverID, hash)
}

func (s *Store) FindByDOI(serverID, doi string) (*Record, error) {
	if doi == "" {
		return nil, nil
	}
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND doi=? LIMIT 1`, serverID, doi)
}

func (s *Store) FindByPMID(serverID, pmid string) (*Record, error) {
	if pmid == "" {
		return nil, nil
	}
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND pmid=? LIMIT 1`, serverID, pmid)
}

func (s *Store) FindByPMCID(serverID, pmcid string) (*Record, error) {
	if pmcid == "" {
		return nil, nil
	}
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND pmcid=? LIMIT 1`, serverID, pmcid)
}

func (s *Store) FindByTitleNorm(serverID, titleNorm string) (*Record, error) {
	if titleNorm == "" {
		return nil, nil
	}
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND title_norm=? LIMIT 1`, serverID, titleNorm)
}

func (s *Store) FindByFamilyKey(serverID, familyKey string) (*Record, error) {
	if familyKey == "" {
		return nil, nil
	}
	return s.scanOne(`SELECT `+selectCols+` FROM ingest_files WHERE server_id=? AND family_key=? LIMIT 1`, serverID, familyKey)
}

func (s *Store) Insert(r Record) error {
	if r.IngestedAt == 0 {
		r.IngestedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO ingest_files
(server_id,file_hash,path,doi,pmid,pmcid,title_norm,family_key,item_key,attachment_key,source,ingested_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ServerID, r.FileHash, r.Path, nullStr(r.DOI), nullStr(r.PMID), nullStr(r.PMCID),
		nullStr(r.TitleNorm), nullStr(r.FamilyKey), r.ItemKey, r.AttachmentKey, r.Source, r.IngestedAt)
	return err
}

func (s *Store) DeleteByItemKey(serverID, itemKey string) error {
	_, err := s.db.Exec(`DELETE FROM ingest_files WHERE server_id=? AND item_key=?`, serverID, itemKey)
	return err
}

func (s *Store) scanOne(q string, args ...any) (*Record, error) {
	row := s.db.QueryRow(q, args...)
	var r Record
	var doi, pmid, pmcid, title, family sql.NullString
	err := row.Scan(&r.ServerID, &r.FileHash, &r.Path, &doi, &pmid, &pmcid, &title, &family,
		&r.ItemKey, &r.AttachmentKey, &r.Source, &r.IngestedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.DOI = doi.String
	r.PMID = pmid.String
	r.PMCID = pmcid.String
	r.TitleNorm = title.String
	r.FamilyKey = family.String
	return &r, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) Ping() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil store")
	}
	return s.db.Ping()
}
