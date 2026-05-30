package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	project    TEXT,
	url        TEXT,
	summary    TEXT NOT NULL,
	ai_tool    TEXT DEFAULT 'claude',
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS todos (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	text       TEXT NOT NULL,
	status     TEXT DEFAULT 'open',
	priority   INTEGER DEFAULT 0,
	session_id INTEGER,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS decisions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	text       TEXT NOT NULL,
	context    TEXT,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT
);
`

type DB struct {
	db  *sql.DB
	dir string
}

func dbDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".ai-context"), nil
}

func openDB() (*DB, error) {
	dir, err := dbDir()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not initialized — run: aictx init")
	}
	return openDBAt(dir)
}

func openDBAt(dir string) (*DB, error) {
	path := filepath.Join(dir, "context.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %v", err)
	}
	return &DB{db: db, dir: dir}, nil
}

func now() int64 { return time.Now().Unix() }

// Sessions

type Session struct {
	ID        int64
	Project   string
	URL       string
	Summary   string
	AITool    string
	CreatedAt int64
}

func (d *DB) SaveSession(project, url, summary, aiTool string) (*Session, error) {
	res, err := d.db.Exec(
		`INSERT INTO sessions (project, url, summary, ai_tool, created_at) VALUES (?,?,?,?,?)`,
		project, url, summary, aiTool, now(),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Session{ID: id, Project: project, URL: url, Summary: summary, AITool: aiTool, CreatedAt: now()}, nil
}

func (d *DB) Sessions(limit int) ([]*Session, error) {
	rows, err := d.db.Query(
		`SELECT id, project, url, summary, ai_tool, created_at FROM sessions ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		s := &Session{}
		rows.Scan(&s.ID, &s.Project, &s.URL, &s.Summary, &s.AITool, &s.CreatedAt)
		out = append(out, s)
	}
	return out, nil
}

// Todos

type Todo struct {
	ID        int64
	Text      string
	Status    string
	Priority  int
	SessionID int64
	CreatedAt int64
	UpdatedAt int64
}

func (d *DB) AddTodo(text string, priority int) (*Todo, error) {
	res, err := d.db.Exec(
		`INSERT INTO todos (text, priority, created_at, updated_at) VALUES (?,?,?,?)`,
		text, priority, now(), now(),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Todo{ID: id, Text: text, Status: "open", Priority: priority}, nil
}

func (d *DB) SetTodoStatus(id int64, status string) error {
	_, err := d.db.Exec(`UPDATE todos SET status=?, updated_at=? WHERE id=?`, status, now(), id)
	return err
}

func (d *DB) Todos(status string) ([]*Todo, error) {
	query := `SELECT id, text, status, priority, created_at, updated_at FROM todos`
	var args []any
	if status != "" {
		query += ` WHERE status=?`
		args = append(args, status)
	}
	query += ` ORDER BY status ASC, priority DESC, id ASC`
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Todo
	for rows.Next() {
		t := &Todo{}
		rows.Scan(&t.ID, &t.Text, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt)
		out = append(out, t)
	}
	return out, nil
}

// Decisions

type Decision struct {
	ID        int64
	Text      string
	Context   string
	CreatedAt int64
}

func (d *DB) AddDecision(text, context string) error {
	_, err := d.db.Exec(`INSERT INTO decisions (text, context, created_at) VALUES (?,?,?)`, text, context, now())
	return err
}

func (d *DB) Decisions() ([]*Decision, error) {
	rows, err := d.db.Query(`SELECT id, text, context, created_at FROM decisions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Decision
	for rows.Next() {
		dec := &Decision{}
		rows.Scan(&dec.ID, &dec.Text, &dec.Context, &dec.CreatedAt)
		out = append(out, dec)
	}
	return out, nil
}

// Meta

func (d *DB) SetMeta(key, val string) error {
	_, err := d.db.Exec(`INSERT INTO meta (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, val)
	return err
}

func (d *DB) GetMeta(key string) string {
	var val string
	d.db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&val)
	return val
}

// Stats

type Stats struct {
	Open       int
	InProgress int
	Done       int
	Blocked    int
	Sessions   int
	Decisions  int
}

func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE status='open'`).Scan(&s.Open)
	d.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE status='in_progress'`).Scan(&s.InProgress)
	d.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE status='done'`).Scan(&s.Done)
	d.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE status='blocked'`).Scan(&s.Blocked)
	d.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&s.Sessions)
	d.db.QueryRow(`SELECT COUNT(*) FROM decisions`).Scan(&s.Decisions)
	return s
}
