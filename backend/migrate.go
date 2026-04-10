package main

import (
	"database/sql"
	_ "embed"
)

//go:embed schema.sql
var ddl string

func migrate(db *sql.DB) error {
	_, err := db.Exec(ddl)
	return err
}
