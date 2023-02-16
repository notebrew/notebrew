package notebrew

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/bokwoon95/sq"
)

func (app *App) Note(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		app.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	segments := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	if segments[0] != "note" || len(segments) > 2 {
		app.Error(w, r, http.StatusNotFound, nil)
		return
	}

	currentUserID, loggedIn := app.CurrentUserID(r)
	if !loggedIn {
		app.Redirect(w, r, "/login", map[string]string{
			"RedirectTo": r.URL.Path,
		})
		return
	}

	if r.Method == "GET" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}
		if len(segments) == 2 {
			noteNumber, err := strconv.Atoi(segments[1])
			if err != nil {
				app.Error(w, r, http.StatusNotFound, nil)
				return
			}
			NOTE := sq.New[NOTE]("")
			body, err := sq.FetchOne(app.DB, sq.
				From(NOTE).
				Where(
					NOTE.USER_ID.EqUUID(currentUserID),
					NOTE.NOTE_NUMBER.EqInt(noteNumber),
				).
				SetDialect(app.Dialect),
				func(row *sq.Row) string {
					return row.StringField(NOTE.BODY)
				},
			)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					app.Error(w, r, http.StatusNotFound, nil)
					return
				}
				app.Error(w, r, http.StatusInternalServerError, err)
				return
			}
			if r.Form.Has("edit") {
				// err := server.Render(w, "html/edit_note.html", nil)
				tmpl, err := template.ParseFiles("html/edit_note.html")
				if err != nil {
					app.Error(w, r, http.StatusInternalServerError, err)
					return
				}
				var buf bytes.Buffer
				err = tmpl.Execute(&buf, nil)
				if err != nil {
					app.Error(w, r, http.StatusInternalServerError, err)
					return
				}
				_, err = buf.WriteTo(w)
				if err != nil {
					log.Println(err)
				}
				return
			}
			_, err = io.WriteString(w, body)
			if err != nil {
				log.Println(err)
			}
			return
		}
		filename := "html/notes.html"
		if r.Form.Has("new") {
			filename = "html/new_note.html"
		}
		tmpl, err := template.ParseFiles(filename)
		if err != nil {
			app.Error(w, r, http.StatusInternalServerError, err)
			return
		}
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, nil)
		if err != nil {
			app.Error(w, r, http.StatusInternalServerError, err)
			return
		}
		_, err = buf.WriteTo(w)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if len(segments) < 2 {
		app.Error(w, r, http.StatusNotFound, nil)
		return
	}
	noteNumber, err := strconv.Atoi(segments[1])
	if err != nil {
		app.Error(w, r, http.StatusNotFound, nil)
		return
	}
	var b strings.Builder
	_, err = io.Copy(&b, r.Body)
	if err != nil {
		app.Error(w, r, http.StatusNotFound, nil)
		return
	}
	body := b.String()
	NOTE := sq.New[NOTE]("")
	insertQuery := sq.InsertQuery{
		Dialect:     app.Dialect,
		InsertTable: NOTE,
		ColumnMapper: func(col *sq.Column) {
			col.SetUUID(NOTE.USER_ID, currentUserID)
			col.SetInt(NOTE.NOTE_NUMBER, noteNumber)
			col.SetString(NOTE.BODY, body)
		},
	}
	switch app.Dialect {
	case sq.DialectSQLite, sq.DialectPostgres:
		insertQuery.Conflict.Fields = sq.Fields{NOTE.USER_ID, NOTE.NOTE_NUMBER}
		insertQuery.Conflict.Resolution = sq.Assignments{
			NOTE.BODY.Set(NOTE.BODY.WithPrefix("EXCLUDED")),
		}
	case sq.DialectMySQL:
		insertQuery.RowAlias = "new"
		insertQuery.Conflict.Resolution = sq.Assignments{
			NOTE.BODY.Set(NOTE.BODY.WithPrefix("new")),
		}
	}
	_, err = sq.Exec(app.DB, insertQuery)
	if err != nil {
		app.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/note/"+strconv.Itoa(noteNumber), http.StatusFound)
}
