package notebrew

import (
	"database/sql"
	"embed"
	"io"
	"io/fs"

	"github.com/bokwoon95/sq"
	"github.com/bokwoon95/sqddl/ddl"
)

//go:embed tables.go mysql_migrations postgres_migrations sqlite_migrations
var migrationFS embed.FS

func Automigrate(dialect string, db *sql.DB) error {
	automigrateCmd := &ddl.AutomigrateCmd{
		DB:             db,
		Dialect:        dialect,
		DirFS:          migrationFS,
		Filenames:      []string{"tables.go"},
		DropObjects:    true,
		AcceptWarnings: true,
		DryRun:         true,
	}
	err := automigrateCmd.Run()
	if err != nil {
		return err
	}
	automigrateCmd.DryRun = false
	automigrateCmd.Stderr = io.Discard
	err = automigrateCmd.Run()
	if err != nil {
		return err
	}
	migrateCmd := &ddl.MigrateCmd{
		DB:      db,
		Dialect: dialect,
	}
	switch dialect {
	case ddl.DialectSQLite:
		fsys, err := fs.Sub(migrationFS, "sqlite_migrations")
		if err != nil {
			return err
		}
		migrateCmd.DirFS = fsys
	case ddl.DialectPostgres:
		fsys, err := fs.Sub(migrationFS, "postgres_migrations")
		if err != nil {
			return err
		}
		migrateCmd.DirFS = fsys
	case ddl.DialectMySQL:
		fsys, err := fs.Sub(migrationFS, "mysql_migrations")
		if err != nil {
			return err
		}
		migrateCmd.DirFS = fsys
	}
	err = migrateCmd.Run()
	if err != nil {
		return err
	}
	return nil
}

type USERS struct {
	sq.TableStruct
	USER_ID       sq.UUIDField   `ddl:"primarykey"`
	EMAIL         sq.StringField `ddl:"unique notnull len=255"`
	NAME          sq.StringField `ddl:"len=255"`
	PASSWORD_HASH sq.StringField `ddl:"len=255"`
}

type NOTE struct {
	sq.TableStruct `ddl:"primarykey=user_id,note_number"`
	USER_ID        sq.UUIDField `ddl:"references={users index}"`
	NOTE_NUMBER    sq.NumberField
	BODY           sq.StringField `ddl:"len=65536"`
	FTS            sq.AnyField    `ddl:"dialect=postgres type=TSVECTOR index={. using=gin}"`
}

type NOTE_FTS struct {
	sq.TableStruct `ddl:"virtual dialect=mysql,sqlite"`
	USER_ID        sq.UUIDField   `ddl:"dialect=mysql"`
	NOTE_NUMBER    sq.NumberField `ddl:"dialect=mysql"`
	BODY           sq.StringField
	NOTE_FTS       sq.AnyField    `ddl:"dialect=sqlite"`
	RANK           sq.NumberField `ddl:"dialect=sqlite"`
	_              struct{}       `ddl:"mysql:primarykey=user_id,note_number"`
	_              struct{}       `ddl:"mysql:index={body using=fulltext}"`
}

type FLASH_SESSION struct {
	sq.TableStruct
	SESSION_ID sq.UUIDField `ddl:"primarykey"`
	VALUE      sq.JSONField
}

type LOGIN_SESSION struct {
	sq.TableStruct
	SESSION_ID sq.UUIDField `ddl:"primarykey"`
	USER_ID    sq.UUIDField
}
