package notebrew

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/bokwoon95/sq"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) Register(w http.ResponseWriter, r *http.Request) {
	const HCaptchaSiteKey = "10000000-ffff-ffff-ffff-000000000001"
	type TemplateData struct {
		Email           string
		HCaptchaSiteKey string
		ErrMsg          string
	}
	type HCaptchaVerificationResult struct {
		// From https://docs.hcaptcha.com/
		// {
		//    "success": true|false,     // is the passcode valid, and does it meet security criteria you specified, e.g. sitekey?
		//    "challenge_ts": timestamp, // timestamp of the challenge (ISO format yyyy-MM-dd'T'HH:mm:ssZZ)
		//    "hostname": string,        // the hostname of the site where the challenge was solved
		//    "credit": true|false,      // optional: whether the response will be credited
		//    "error-codes": [...]       // optional: any error codes
		//    "score": float,            // ENTERPRISE feature: a score denoting malicious activity.
		//    "score_reason": [...]      // ENTERPRISE feature: reason(s) for score.
		// }
		Success     bool   `json:"success"`
		ChallengeTs string `json:"challenge_ts"`
		Hostname    string `json:"hostname"`
		Credit      bool   `json:"credit"`
		ErrorCodes  []any  `json:"error-codes"`
	}

	if r.Method != "GET" && r.Method != "POST" {
		server.Error(w, r, http.StatusMethodNotAllowed, nil)
		return
	}

	// If already logged in, redirect user.
	currentUserID, loggedIn := server.CurrentUserID(r)
	if loggedIn {
		server.Redirect(w, r, "/user/"+strings.ToLower(currentUserID.String()), nil)
		return
	}

	// If GET, render registration page.
	if r.Method == "GET" {
		var templateData TemplateData
		err := server.Flash(w, r, &templateData)
		if err != nil {
			log.Println(err)
		}
		templateData.HCaptchaSiteKey = HCaptchaSiteKey
		tmpl, err := template.ParseFiles("html/register.html")
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
		Email: r.PostForm.Get("email"),
	}
	hCaptchaResponse := r.PostForm.Get("h-captcha-response")
	password := r.PostForm.Get("password")

	// Check with hCaptcha if the captcha response is valid.
	resp, err := http.PostForm("https://hcaptcha.com/siteverify", url.Values{
		"secret":   []string{"0x0000000000000000000000000000000000000000"},
		"response": []string{hCaptchaResponse},
		"sitekey":  []string{HCaptchaSiteKey},
	})
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	defer resp.Body.Close()
	var verificationResult HCaptchaVerificationResult
	err = json.NewDecoder(resp.Body).Decode(&verificationResult)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	if !verificationResult.Success {
		templateData.ErrMsg = "failed captcha"
		server.Redirect(w, r, r.URL.Path, templateData)
		return
	}

	// Create user.
	b, err := bcrypt.GenerateFromPassword([]byte(password), 11)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	passwordHash := string(b)
	USERS := sq.New[USERS]("")
	result, err := sq.ExecContext(r.Context(), server.DB, sq.
		Update(USERS).
		Set(USERS.PASSWORD_HASH.SetString(passwordHash)).
		Where(USERS.EMAIL.EqString(templateData.Email)).
		SetDialect(server.Dialect),
	)
	if err != nil {
		server.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	if result.RowsAffected == 0 {
		userID := [16]byte(ulid.Make())
		_, err := sq.ExecContext(r.Context(), server.DB, sq.
			InsertInto(USERS).
			ColumnValues(func(col *sq.Column) {
				col.SetUUID(USERS.USER_ID, userID)
				col.SetString(USERS.EMAIL, templateData.Email)
				col.SetString(USERS.PASSWORD_HASH, passwordHash)
			}).
			SetDialect(server.Dialect),
		)
		if err != nil {
			server.Error(w, r, http.StatusInternalServerError, err)
			return
		}
	}

	server.Redirect(w, r, "/login", map[string]string{
		"Email": templateData.Email,
	})
}
