package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bokwoon95/sqddl/ddl"
	"github.com/bokwoon95/sqddl/drivers/ddlpostgres"
	"github.com/bokwoon95/sqddl/drivers/ddlsqlite3"
	"github.com/notebrew/notebrew"
)

var (
	dsn = flag.String("db", "notebrew-data/notebrew.db", "Data Source Name")
)

func init() {
	flag.Parse()
	ddlpostgres.Register()
	ddlsqlite3.Register()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	dialect, driverName, normalizedDSN := ddl.NormalizeDSN(*dsn)
	if dialect == "sqlite" {
		err := os.MkdirAll(filepath.Dir(normalizedDSN), 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
	db, err := sql.Open(driverName, normalizedDSN)
	if err != nil {
		log.Fatal(err)
	}
	err = notebrew.Automigrate(dialect, db)
	if err != nil {
		log.Fatal(err)
	}
	server := &notebrew.Server{
		DB:      db,
		Dialect: dialect,
		NoteFS:  notebrew.NestedDirFS("notebrew-data/note"),
		ImageFS: notebrew.NestedDirFS("notebrew-data/image"),
	}
	fmt.Println("Listening on localhost:7070")
	http.ListenAndServe("localhost:7070", server.Handler())
}
