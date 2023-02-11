package notebrew

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type Server struct {
	DB         *sql.DB
	Dialect    string
	DataDir    fs.FS
	SigningKey [32]byte
	Notes      Store
	// TODO: figure out the proper interface for a writable FS. And then create
	// a custom filesystem wrapper that additionally looks up a folder first
	// using the last 2 characters of a file id before locating the file
	// itself.
}

func (server *Server) Redirect(w http.ResponseWriter, r *http.Request, destURL string, data any) {
	if data == nil {
		http.Redirect(w, r, destURL, http.StatusFound)
		return
	}
	u, err := url.Parse(destURL)
	if err != nil {
		panic(err)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	base64Payload := base64.URLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, server.SigningKey[:])
	mac.Write([]byte(base64Payload))
	signature := mac.Sum(nil)
	base64Signature := base64.URLEncoding.EncodeToString(signature)
	http.SetCookie(w, &http.Cookie{
		Name:     base64.URLEncoding.EncodeToString([]byte(u.Path)),
		Value:    base64Signature + "." + base64Payload,
		MaxAge:   3,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, destURL, http.StatusFound)
}

func (server *Server) Flash(w http.ResponseWriter, r *http.Request, dest any) error {
	name := base64.URLEncoding.EncodeToString([]byte(r.URL.Path))
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		MaxAge: -1,
	})
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil
	}
	base64Signature, base64Payload, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return nil
	}
	gotSignature, err := base64.URLEncoding.DecodeString(base64Signature)
	if err != nil {
		return nil
	}
	mac := hmac.New(sha256.New, server.SigningKey[:])
	mac.Write([]byte(base64Payload))
	wantSignature := mac.Sum(nil)
	if !hmac.Equal(gotSignature, wantSignature) {
		return nil
	}
	payload, err := base64.URLEncoding.DecodeString(base64Payload)
	if err != nil {
		return nil
	}
	return json.Unmarshal(payload, dest)
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

func (server *Server) Error(w http.ResponseWriter, r *http.Request, code int, msg any) {
	_, file, line, _ := runtime.Caller(1)
	data := map[string]any{
		"Title":  strconv.Itoa(code) + " " + http.StatusText(code),
		"Caller": file + ":" + strconv.Itoa(line),
		"Msg":    msg,
		"Code":   code,
	}
	if r.Method == "GET" {
		_ = errTemplate.Execute(w, data)
		return
	}
	server.Redirect(w, r, "/error", data)
}

func (server *Server) ErrorPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	server.Flash(w, r, &data)
	code, ok := data["Code"].(int)
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.WriteHeader(code)
	_ = errTemplate.Execute(w, data)
}

// go:embed static
var staticFS embed.FS

var rootFS = os.DirFS(".")

func (server *Server) Static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	file, err := rootFS.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			server.Error(w, r, http.StatusNotFound, nil)
			return
		}
		server.Error(w, r, http.StatusInternalServerError, err)
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
	var modtime time.Time
	fileinfo, err := file.Stat()
	if err == nil {
		modtime = fileinfo.ModTime()
	}
	fileseeker, ok := file.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, name, modtime, fileseeker)
		return
	}
	var buf bytes.Buffer
	buf.Grow(int(fileinfo.Size()))
	_, err = buf.ReadFrom(file)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	http.ServeContent(w, r, name, modtime, bytes.NewReader(buf.Bytes()))
}

func (server *Server) CurrentUserID(r *http.Request) (ulid.ULID, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ulid.ULID{}, false
	}
	base64Signature, sessionToken, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return ulid.ULID{}, false
	}
	gotSignature, err := base64.URLEncoding.DecodeString(base64Signature)
	if err != nil {
		return ulid.ULID{}, false
	}
	mac := hmac.New(sha256.New, server.SigningKey[:])
	mac.Write([]byte(sessionToken))
	wantSignature := mac.Sum(nil)
	if !hmac.Equal(gotSignature, wantSignature) {
		return ulid.ULID{}, false
	}
	createdAtString, userIDString, ok := strings.Cut(sessionToken, ".")
	if !ok {
		return ulid.ULID{}, false
	}
	createdAt, err := time.Parse("20060102", createdAtString)
	if err != nil {
		return ulid.ULID{}, false
	}
	if time.Now().Sub(createdAt) > time.Hour*24*30 {
		return ulid.ULID{}, false
	}
	userID, err := ulid.Parse(userIDString)
	if err != nil {
		return ulid.ULID{}, false
	}
	return userID, true
}

type Store interface {
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
	Remove(name string) error
}

type dirStore string

func DirStore(dir string) Store {
	return dirStore(dir)
}

func (dir dirStore) Read(name string) ([]byte, error) {
	if len(name) != 26 {
		return nil, fmt.Errorf("invalid name length")
	}
	return os.ReadFile(path.Join(string(dir), name[len(name)-2:], name+".txt"))
}

func (dir dirStore) Write(name string, data []byte) error {
	if len(name) != 26 {
		return fmt.Errorf("invalid name length")
	}
	return os.WriteFile(path.Join(string(dir), name[len(name)-2:], name+".txt"), data, 0644)
}

func (dir dirStore) Remove(name string) error {
	if len(name) != 26 {
		return fmt.Errorf("invalid name length")
	}
	return os.Remove(path.Join(string(dir), name[len(name)-2:], name+".txt"))
}
