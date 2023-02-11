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
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}
		if len(segments) == 2 {
			noteNumber := segments[1]
			_, err := strconv.Atoi(noteNumber)
			if err != nil {
				server.Error(w, r, http.StatusNotFound, nil)
				return
			}
			noteID := strings.ToLower(currentUserID.String()) + "-" + noteNumber
			file, err := server.NoteFS.Open(noteID)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					server.Error(w, r, http.StatusNotFound, nil)
					return
				}
				server.Error(w, r, http.StatusInternalServerError, err)
				return
			}
			if r.Form.Has("edit") {
				// err := server.Render(w, "html/edit_note.html", nil)
				tmpl, err := template.ParseFiles("html/edit_note.html")
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
			_, err = io.Copy(w, file)
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
	noteNumber := segments[1]
	_, err := strconv.Atoi(noteNumber)
	if err != nil {
		server.Error(w, r, http.StatusNotFound, nil)
		return
	}
	noteID := strings.ToLower(currentUserID.String()) + "-" + noteNumber
	writer, err := server.NoteFS.OpenWriter(noteID)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	defer writer.Close()
	_, err = io.Copy(writer, r.Body)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	err = writer.Close()
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/note/"+noteNumber, http.StatusFound)
}
