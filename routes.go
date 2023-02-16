package notebrew

import (
	"net/http"
	"strings"
)

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/error", app.ErrorPage)
	mux.HandleFunc("/login", app.Login)
	mux.HandleFunc("/logout", app.Logout)
	mux.HandleFunc("/register", app.Register)
	mux.HandleFunc("/user/", app.User)
	mux.HandleFunc("/u/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user/"+strings.TrimPrefix(r.URL.Path, "/u/"), http.StatusFound)
	})
	mux.HandleFunc("/note/", app.Note)
	mux.HandleFunc("/n/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/note/"+strings.TrimPrefix(r.URL.Path, "/n/"), http.StatusFound)
	})
	mux.HandleFunc("/static/", app.Static)
	mux.HandleFunc("/esmodules/", app.Static)
	mux.HandleFunc("/", app.Root)
	return mux
}
