package main

import (
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bokwoon95/sqddl/ddl"
	"github.com/bokwoon95/sqddl/drivers/ddlpostgres"
	"github.com/bokwoon95/sqddl/drivers/ddlsqlite3"
	"github.com/notebrew/notebrew"
	"golang.org/x/crypto/hkdf"
)

var (
	dsn       = flag.String("db", "notebrew.db", "Data Source Name")
	secretKey = flag.String("key", "lorem ipsum dolor sit amet", "Secret Key")
)

func init() {
	flag.Parse()
	ddlpostgres.Register()
	ddlsqlite3.Register()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	dialect, driverName, normalizedDSN := ddl.NormalizeDSN(*dsn)
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
		Notes:   notebrew.DirStore("note"),
	}
	kdf := hkdf.New(sha256.New, []byte(*secretKey), nil, nil)
	_, err = io.ReadFull(kdf, server.SigningKey[:])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Listening on localhost:7070")
	http.ListenAndServe("localhost:7070", server.Handler())
}
