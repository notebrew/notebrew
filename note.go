package notebrew

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
)

func (server *Server) Note(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	segments := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	if segments[0] != "note" || len(segments) > 2 {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}

	currentUserID, loggedIn := server.CurrentUserID(r)
	if !loggedIn {
		server.Redirect(w, r, "/login", map[string]string{
			"RedirectTo": r.URL.Path,
		})
		return
	}

	if r.Method == "GET" {
		if len(segments) == 2 {
			noteID := segments[1]
			_, err := strconv.Atoi(noteID)
			if err != nil {
				server.Error(w, r, http.StatusNotFound, nil)
				return
			}
			b, err := server.Notes.Read(path.Join(strings.ToLower(currentUserID.String()), noteID))
			_, err = w.Write(b)
			if err != nil {
				log.Println(err)
			}
			return
		}
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}
		isNew := r.Form.Has("new")
		isEdit := r.Form.Has("edit")
		filename := "html/notes.html"
		if isNew {
			filename = "html/new_note.html"
		} else if isEdit {
			filename = "html/edit_note.html"
		}
		tmpl, err := template.ParseFiles(filename)
		if err != nil {
			server.Error(w, r, http.StatusInternalServerError, err)
			return
		}
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, nil)
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

	if len(segments) < 2 {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}
	noteID := segments[1]
	_, err := strconv.Atoi(noteID)
	if err != nil {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	err = server.Notes.Write(path.Join(strings.ToLower(currentUserID.String()), noteID), buf.Bytes())
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/note/"+noteID, http.StatusFound)
}
