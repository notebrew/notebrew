package notebrew

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bokwoon95/sq"
	"github.com/bokwoon95/sqddl/ddl"
	"github.com/bokwoon95/sqddl/drivers/ddlmysql"
	"github.com/bokwoon95/sqddl/drivers/ddlpostgres"
	"github.com/bokwoon95/sqddl/drivers/ddlsqlite3"
	"github.com/oklog/ulid/v2"
)

func init() {
	ddlsqlite3.Register()
	ddlpostgres.Register()
	ddlmysql.Register()
}

type App struct {
	DB      *sql.DB
	Dialect string
	ImageFS FS
}

func NewApp(databaseURL string, dataDir string) (*App, error) {
	if dataDir == "" {
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(userHomeDir, "notebrewdata")
	}
	if databaseURL == "" {
		err := os.MkdirAll(dataDir, 0755)
		if err != nil {
			return nil, err
		}
		// See https://github.com/mattn/go-sqlite3#connection-string for
		// available options.
		options := url.Values{
			"_foreign_keys": []string{"1"},
			"_journal_mode": []string{"WAL"},
			"_txlock":       []string{"immediate"},
			"_busy_timeout": []string{strconv.FormatInt((10 * time.Second).Milliseconds(), 10)},
		}
		databaseURL = filepath.Join(dataDir, "notebrew.db?"+options.Encode())
	}
	dialect, driverName, dataSourceName := ddl.NormalizeDSN(databaseURL)
	if dialect == ddl.DialectSQLite {
		err := os.MkdirAll(filepath.Dir(dataSourceName), 0755)
		if err != nil {
			return nil, err
		}
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	err = Automigrate(dialect, db)
	if err != nil {
		return nil, err
	}
	imageDir := filepath.Join(dataDir, "image")
	err = os.MkdirAll(imageDir, 0755)
	if err != nil {
		return nil, err
	}
	app := &App{
		DB:      db,
		Dialect: dialect,
		ImageFS: NestedDirFS(imageDir),
	}
	return app, nil
}

func (app *App) Cleanup() error {
	return app.DB.Close()
}

func (app *App) Redirect(w http.ResponseWriter, r *http.Request, destURL string, data any) {
	if data == nil {
		http.Redirect(w, r, destURL, http.StatusFound)
		return
	}
	u, err := url.Parse(destURL)
	if err != nil {
		panic(err)
	}
	var b strings.Builder
	err = json.NewEncoder(&b).Encode(data)
	if err != nil {
		panic(err)
	}
	value := b.String()
	sessionID := ulid.Make()
	FLASH_SESSION := sq.New[FLASH_SESSION]("")
	_, err = sq.Exec(app.DB, sq.
		InsertInto(FLASH_SESSION).
		ColumnValues(func(col *sq.Column) {
			col.SetUUID(FLASH_SESSION.SESSION_ID, sessionID)
			col.SetString(FLASH_SESSION.VALUE, value)
		}).
		SetDialect(app.Dialect),
	)
	if err != nil {
		log.Println(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     base64.URLEncoding.EncodeToString([]byte(u.Path)),
		Value:    strings.ToLower(sessionID.String()),
		MaxAge:   3,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, destURL, http.StatusFound)
}

func (app *App) Flash(w http.ResponseWriter, r *http.Request, dest any) error {
	name := base64.URLEncoding.EncodeToString([]byte(r.URL.Path))
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		MaxAge: -1,
	})
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil
	}
	FLASH_SESSION := sq.New[FLASH_SESSION]("")
	sessionID, err := ulid.Parse(cookie.Value)
	if err != nil {
		return nil
	}
	defer func() {
		_, err := sq.Exec(app.DB, sq.
			DeleteFrom(FLASH_SESSION).
			Where(FLASH_SESSION.SESSION_ID.EqUUID(sessionID)).
			SetDialect(app.Dialect),
		)
		if err != nil {
			log.Println(err)
		}
	}()
	value, err := sq.FetchOne(app.DB, sq.
		From(FLASH_SESSION).
		Where(FLASH_SESSION.SESSION_ID.EqUUID(sessionID)),
		func(row *sq.Row) []byte {
			return row.BytesField(FLASH_SESSION.VALUE)
		},
	)
	return json.Unmarshal(value, dest)
}

var errTemplate = template.Must(template.New("error").Parse(`<!DOCTYPE html>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://unpkg.com/tachyons/css/tachyons.min.css">
<link rel="stylesheet" href="/static/styles.css">
<title>{{ .Title }}</title>
<h1>{{ .Title }}</h1>
{{ with .Caller }}<pre>{{ . }}</pre>{{ end }}
{{ with .Msg }}<pre>{{ . }}</pre>{{ end }}
`))

func (app *App) Error(w http.ResponseWriter, r *http.Request, code int, msg any) {
	_, file, line, _ := runtime.Caller(1)
	data := map[string]any{
		"Title":  strconv.Itoa(code) + " " + http.StatusText(code),
		"Caller": file + ":" + strconv.Itoa(line),
		"Msg":    msg,
		"Code":   code,
	}
	if r.Method == "GET" {
		w.WriteHeader(code)
		_ = errTemplate.Execute(w, data)
		return
	}
	app.Redirect(w, r, "/error", data)
}

func (app *App) ErrorPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	app.Flash(w, r, &data)
	code, ok := data["Code"].(int)
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.WriteHeader(code)
	_ = errTemplate.Execute(w, data)
}

// go:embed static esmodules
var staticFS embed.FS

var rootFS = os.DirFS(".")

func (app *App) Static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	file, err := rootFS.Open(strings.TrimSuffix(name, "/"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			app.Error(w, r, http.StatusNotFound, nil)
			return
		}
		app.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	fileinfo, err := file.Stat()
	if err != nil {
		app.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	if fileinfo.IsDir() {
		app.Error(w, r, http.StatusNotFound, nil)
		return
	}
	if strings.HasSuffix(name, ".gz") {
		ext := path.Ext(strings.TrimSuffix(name, ".gz"))
		if ext != "" {
			mimeType := mime.TypeByExtension(ext)
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Encoding", "gzip")
		}
	}
	fileseeker, ok := file.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, name, fileinfo.ModTime(), fileseeker)
		return
	}
	var buf bytes.Buffer
	buf.Grow(int(fileinfo.Size()))
	_, err = buf.ReadFrom(file)
	if err != nil {
		app.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.ServeContent(w, r, name, fileinfo.ModTime(), bytes.NewReader(buf.Bytes()))
}

func (app *App) CurrentUserID(r *http.Request) (ulid.ULID, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ulid.ULID{}, false
	}
	sessionID, err := ulid.Parse(cookie.Value)
	if err != nil {
		return ulid.ULID{}, false
	}
	LOGIN_SESSION := sq.New[LOGIN_SESSION]("")
	userID, err := sq.FetchOne(app.DB, sq.
		From(LOGIN_SESSION).
		Where(LOGIN_SESSION.SESSION_ID.EqUUID(sessionID)).
		SetDialect(app.Dialect),
		func(row *sq.Row) (userID ulid.ULID) {
			row.UUIDField(&userID, LOGIN_SESSION.USER_ID)
			return userID
		},
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println(err)
		}
		return ulid.ULID{}, false
	}
	return userID, true
}

type FS interface {
	Open(name string) (fs.File, error)
	OpenWriter(name string) (io.WriteCloser, error)
}

type dirFS struct {
	dir    string
	nested bool
}

func DirFS(dir string) FS {
	return dirFS{dir: dir}
}

func NestedDirFS(dir string) FS {
	return dirFS{dir: dir, nested: true}
}

func (d dirFS) Open(name string) (fs.File, error) {
	if len(name) < ulid.EncodedSize {
		return nil, fmt.Errorf("invalid name")
	}
	if d.nested {
		name = path.Join(name[ulid.EncodedSize-2:ulid.EncodedSize], name)
	}
	return os.Open(path.Join(d.dir, name))
}

func (d dirFS) OpenWriter(name string) (io.WriteCloser, error) {
	if len(name) < ulid.EncodedSize {
		return nil, fmt.Errorf("invalid name")
	}
	if d.nested {
		name = path.Join(name[ulid.EncodedSize-2:ulid.EncodedSize], name)
	}
	err := os.MkdirAll(d.dir, 0755)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(path.Join(d.dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}
