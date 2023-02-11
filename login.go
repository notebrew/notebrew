package notebrew

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

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
	userIDString := strings.ToLower(result.UserID.String())
	sessionToken := time.Now().UTC().Format("20060102") + "." + userIDString
	mac := hmac.New(sha256.New, server.SigningKey[:])
	mac.Write([]byte(sessionToken))
	signature := mac.Sum(nil)
	base64Signature := base64.URLEncoding.EncodeToString(signature)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    base64Signature + "." + sessionToken,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	if redirectTo != "" {
		http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/user/"+userIDString, http.StatusFound)
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
	http.Redirect(w, r, "/", http.StatusFound)
}
