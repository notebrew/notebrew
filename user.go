package notebrew

import (
	"bytes"
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/bokwoon95/sq"
	"github.com/oklog/ulid/v2"
)

func (server *Server) User(w http.ResponseWriter, r *http.Request) {
	type TemplateData struct {
		UserID        string
		CurrentUserID string
		Name          string
	}

	if r.Method != "GET" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	segments := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	if segments[0] != "user" || len(segments) > 2 {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}

	currentUserID, loggedIn := server.CurrentUserID(r)
	if len(segments) < 2 {
		if loggedIn {
			http.Redirect(w, r, "/user/"+strings.ToLower(currentUserID.String()), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	base32UserID := segments[1]
	if len(base32UserID) != ulid.EncodedSize {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}

	// Normalize uppercase base32UserID to lowercase.
	hasUppercase := false
	for _, char := range base32UserID {
		if 'A' <= char && char <= 'Z' {
			hasUppercase = true
			break
		}
	}
	if hasUppercase {
		http.Redirect(w, r, "/user/"+strings.ToLower(base32UserID), http.StatusFound)
		return
	}

	// Decode userID.
	userID, err := ulid.Parse(base32UserID)
	if err != nil {
		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			MaxAge: -1,
		})
		server.Error(w, r, http.StatusNotFound, nil)
	}

	// Fetch user by userID.
	USERS := sq.New[USERS]("")
	name, err := sq.FetchOneContext(r.Context(), server.DB, sq.
		From(USERS).
		Where(USERS.USER_ID.EqUUID(userID)).
		SetDialect(server.Dialect),
		func(row *sq.Row) string {
			return row.StringField(USERS.NAME)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.SetCookie(w, &http.Cookie{
				Name:   "session",
				MaxAge: -1,
			})
			server.Error(w, r, http.StatusNotFound, nil)
			return
		}
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}

	// Render user page.
	templateData := TemplateData{
		UserID:        strings.ToLower(userID.String()),
		CurrentUserID: strings.ToLower(currentUserID.String()),
		Name:          name,
	}
	tmpl, err := template.ParseFiles("html/user.html")
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	_, err = buf.WriteTo(w)
	if err != nil {
		log.Println(err)
	}
}
