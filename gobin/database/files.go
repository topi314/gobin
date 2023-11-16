package database

type File struct {
	ID         string `db:"id"`
	DocumentID string `db:"document_id"`
	Name       string `db:"name"`
	Version    int64  `db:"version"`
	Content    string `db:"content"`
	Language   string `db:"language"`
}
