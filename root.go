package notebrew

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/oklog/ulid/v2"
)

func (server *Server) Root(w http.ResponseWriter, r *http.Request) {
	type TemplateData struct {
		UserID     string
		LoggedIn bool
	}

	if r.Method != "GET" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	segments := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	if len(segments) > 1 {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}

	// Render home page.
	if segments[0] == "" {
		var templateData TemplateData
		_, templateData.LoggedIn = server.CurrentUserID(r)
		tmpl, err := template.ParseFiles("html/home.html")
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

	ID := segments[0]
	isNumeric := true
	isAlphanumeric := true
	for _, char := range ID {
		if '0' <= char && char <= '9' {
			continue
		}
		if ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z') {
			isNumeric = false
			continue
		}
		isNumeric = false
		isAlphanumeric = false
		break
	}

	if isNumeric {
		http.Redirect(w, r, "/note/"+ID, http.StatusFound)
		return
	}

	if isAlphanumeric && len(ID) == ulid.EncodedSize {
		http.Redirect(w, r, "/user/"+ID, http.StatusFound)
		return
	}

	server.Error(w, r, http.StatusNotFound, nil)
	return
}
