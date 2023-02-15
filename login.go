package notebrew

import (
	"bytes"
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/bokwoon95/sq"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) Login(w http.ResponseWriter, r *http.Request) {
	type TemplateData struct {
		Email      string
		ErrMsg     string
		RedirectTo string
	}

	if r.Method != "GET" && r.Method != "POST" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	currentUserID, loggedIn := server.CurrentUserID(r)
	if loggedIn {
		http.Redirect(w, r, "/user/"+strings.ToLower(currentUserID.String()), http.StatusFound)
		return
	}

	// Render login page.
	if r.Method == "GET" {
		var templateData TemplateData
		err := server.Flash(w, r, &templateData)
		if err != nil {
			log.Println(err)
		}
		tmpl, err := template.ParseFiles("html/login.html")
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
		return
	}

	// Map form data.
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	templateData := TemplateData{
		Email:      r.PostForm.Get("email"),
		RedirectTo: r.PostForm.Get("redirect_to"),
	}
	password := r.PostForm.Get("password")
	redirectTo := r.PostForm.Get("redirect_to")

	// Get user info.
	USERS := sq.New[USERS]("")
	result, err := sq.FetchOneContext(r.Context(), server.DB, sq.
		From(USERS).
		Where(USERS.EMAIL.EqString(templateData.Email)).
		SetDialect(server.Dialect),
		func(row *sq.Row) (result struct {
			UserID       ulid.ULID
			PasswordHash string
		}) {
			row.UUIDField(&result.UserID, USERS.USER_ID)
			result.PasswordHash = row.StringField(USERS.PASSWORD_HASH)
			return result
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		templateData.ErrMsg = "incorrect email or password"
		server.Redirect(w, r, r.URL.Path, templateData)
		return
	}
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}

	// Check if password matches.
	err = bcrypt.CompareHashAndPassword([]byte(result.PasswordHash), []byte(password))
	if err != nil {
		templateData.ErrMsg = "incorrect email or password"
		server.Redirect(w, r, r.URL.Path, templateData)
		return
	}

	// Set session token.
	sessionID := strings.ToLower(ulid.Make().String())
	SESSION := sq.New[SESSION]("")
	_, err = sq.Exec(server.DB, sq.
		InsertInto(SESSION).
		ColumnValues(func(col *sq.Column) {
			col.SetString(SESSION.SESSION_ID, sessionID)
			col.SetUUID(SESSION.USER_ID, result.UserID)
		}).
		SetDialect(server.Dialect),
	)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	if redirectTo != "" {
		http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/user/"+strings.ToLower(result.UserID.String()), http.StatusFound)
}

func (server *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		MaxAge: -1,
	})
	cookie, err := r.Cookie("session")
	if err == nil {
		SESSION := sq.New[SESSION]("")
		sessionID := cookie.Value
		_, err := sq.Exec(server.DB, sq.
			DeleteFrom(SESSION).
			Where(SESSION.SESSION_ID.EqString(sessionID)).
			SetDialect(server.Dialect),
		)
		if err != nil {
			log.Println(err)
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
