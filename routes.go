package notebrew

import (
	"net/http"
	"strings"
)

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/error", server.ErrorPage)
	mux.HandleFunc("/login", server.Login)
	mux.HandleFunc("/logout", server.Logout)
	mux.HandleFunc("/register", server.Register)
	mux.HandleFunc("/user/", server.User)
	mux.HandleFunc("/u/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user/"+strings.TrimPrefix(r.URL.Path, "/u/"), http.StatusFound)
	})
	mux.HandleFunc("/note/", server.Note)
	mux.HandleFunc("/n/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/note/"+strings.TrimPrefix(r.URL.Path, "/n/"), http.StatusFound)
	})
	mux.HandleFunc("/static/", server.Static)
	mux.HandleFunc("/es_modules/", server.Static)
	mux.HandleFunc("/", server.Root)
	return mux
}
