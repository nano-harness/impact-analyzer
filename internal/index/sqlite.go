package index

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// SQLiteStore implements IndexStore using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-based index store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.createTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS file_symbols (
		file TEXT PRIMARY KEY,
		language TEXT,
		package TEXT,
		symbols TEXT,
		imports TEXT,
		content_hash TEXT
	);

	CREATE TABLE IF NOT EXISTS call_sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file TEXT,
		line INTEGER,
		caller_name TEXT,
		callee_name TEXT,
		resolved BOOLEAN
	);

	CREATE INDEX IF NOT EXISTS idx_callee ON call_sites(callee_name);
	CREATE INDEX IF NOT EXISTS idx_caller ON call_sites(caller_name);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) StoreFileSymbols(fileSymbols *model.FileSymbols) error {
	symbolsJSON, err := json.Marshal(fileSymbols.Symbols)
	if err != nil {
		return err
	}

	importsJSON, err := json.Marshal(fileSymbols.Imports)
	if err != nil {
		return err
	}

	// #15: Store content hash for file-level caching
	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO file_symbols (file, language, package, symbols, imports, content_hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`, fileSymbols.File, fileSymbols.Language, fileSymbols.Package, symbolsJSON, importsJSON, fileSymbols.ContentHash)

	return err
}

func (s *SQLiteStore) GetFileSymbols(filePath string) (*model.FileSymbols, error) {
	var fs model.FileSymbols
	var symbolsJSON, importsJSON string

	// #15: Retrieve content hash for file-level caching
	err := s.db.QueryRow(`
		SELECT file, language, package, symbols, imports, COALESCE(content_hash, '')
		FROM file_symbols WHERE file = ?
	`, filePath).Scan(&fs.File, &fs.Language, &fs.Package, &symbolsJSON, &importsJSON, &fs.ContentHash)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(symbolsJSON), &fs.Symbols); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(importsJSON), &fs.Imports); err != nil {
		return nil, err
	}

	return &fs, nil
}

func (s *SQLiteStore) StoreCallSite(callSite *model.CallSite) error {
	callerName := ""
	if callSite.Caller != nil {
		callerName = callSite.Caller.QualifiedName()
	}

	calleeName := ""
	if callSite.Callee != nil {
		calleeName = callSite.Callee.QualifiedName()
	}

	_, err := s.db.Exec(`
		INSERT INTO call_sites (file, line, caller_name, callee_name, resolved)
		VALUES (?, ?, ?, ?, ?)
	`, callSite.File, callSite.Line, callerName, calleeName, callSite.Resolved)

	return err
}

func (s *SQLiteStore) GetCallSitesInFile(filePath string) ([]*model.CallSite, error) {
	rows, err := s.db.Query(`
		SELECT file, line, caller_name, callee_name, resolved
		FROM call_sites WHERE file = ?
	`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var callSites []*model.CallSite
	for rows.Next() {
		var cs model.CallSite
		var callerName, calleeName string
		if err := rows.Scan(&cs.File, &cs.Line, &callerName, &calleeName, &cs.Resolved); err != nil {
			return nil, err
		}

		cs.Caller = &model.Symbol{Name: callerName}
		cs.Callee = &model.Symbol{Name: calleeName}
		callSites = append(callSites, &cs)
	}

	return callSites, rows.Err()
}

func (s *SQLiteStore) GetCallersOf(symbolName string) ([]*model.Symbol, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT caller_name FROM call_sites WHERE callee_name = ?
	`, symbolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var callers []*model.Symbol
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		callers = append(callers, &model.Symbol{Name: name})
	}

	return callers, rows.Err()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
