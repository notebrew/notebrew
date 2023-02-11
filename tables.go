package notebrew

import (
	"database/sql"
	"embed"
	"io"

	"github.com/bokwoon95/sq"
	"github.com/bokwoon95/sqddl/ddl"
)

//go:embed tables.go
var tablesFS embed.FS

func Automigrate(dialect string, db *sql.DB) error {
	automigrateCmd := &ddl.AutomigrateCmd{
		DB:             db,
		Dialect:        dialect,
		DirFS:          tablesFS,
		Filenames:      []string{"tables.go"},
		Stderr:         io.Discard,
		DropObjects:    true,
		AcceptWarnings: true,
	}
	return automigrateCmd.Run()
}

type USERS struct {
	sq.TableStruct
	USER_ID       sq.UUIDField   `ddl:"primarykey"`
	EMAIL         sq.StringField `ddl:"unique notnull"`
	NAME          sq.StringField
	PASSWORD_HASH sq.StringField
}

type NOTE struct {
	sq.TableStruct
	NOTE_ID sq.UUIDField `ddl:"primarykey"`
	USER_ID sq.UUIDField `ddl:"references=users index"`
}
