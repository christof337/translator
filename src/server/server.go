package server

import (
	"../config"
	"../model"
	"fmt"
	// "io/ioutil"
	"../control"
	"golang.org/x/crypto/bcrypt"
	"github.com/bpowers/seshcookie"
	"html/template"
	"net/http"
	"net/smtp"
	"strings"
	"strconv"
)

const SESSIONKEY = "93Yb8c59aASAf3kfT5xU8wz2GmfP4CbSNdhuvLxAdUqZnThbxuAAZu5AVWUrpsmXz47SYnvDcqr7TfNgLP8CpEpAmzGXNvMu72Scd4EAZGuepTQ7kWENemqr"

func RunTranslator(host string, debug int) {
	control.Hostname = host
	model.Debug = debug

	fmt.Println("Starting web server:", control.Hostname)

	handler := http.NewServeMux()
	handler.HandleFunc("/home", control.DashboardHandler)
	handler.HandleFunc("/sources", control.SourcesHandler)
	handler.HandleFunc("/entries", control.EntriesHandler)
	handler.HandleFunc("/translate", control.TranslationHandler)
	handler.HandleFunc("/import", control.ImportHandler)
	handler.HandleFunc("/import/progress", control.ImportProgressHandler)
	handler.HandleFunc("/import/abort", control.ImportAbortHandler)
	handler.HandleFunc("/export", control.ExportHandler)
	handler.HandleFunc("/live-export", control.LiveExportHandler)
	handler.HandleFunc("/users", control.UsersHandler)
	handler.HandleFunc("/users/add", control.UsersAddHandler)
	handler.HandleFunc("/users/del", control.UsersDelHandler)
	handler.HandleFunc("/users/masq", control.UsersMasqueradeHandler)
	handler.HandleFunc("/users/reinvite", control.UsersReinviteHandler)
	handler.HandleFunc("/account", control.AccountHandler)
	handler.HandleFunc("/account/password", control.SetPasswordHandler)
	handler.HandleFunc("/account/reclaim", control.AccountReclaimHandler)

	handler.HandleFunc("/api/setlead", control.APISetLeadHandler)
	handler.HandleFunc("/api/clearlead", control.APIClearLeadHandler)
	handler.HandleFunc("/api/entries", control.APIEntriesHandler)
	handler.HandleFunc("/api/translate", control.APITranslateHandler)
	handler.HandleFunc("/api/vote", control.APIVoteHandler)
	handler.HandleFunc("/api/lookup", control.APILookupHandler)

	handler.Handle("/css/", http.FileServer(http.Dir("web")))
	handler.Handle("/bootstrap/", http.FileServer(http.Dir("web")))
	handler.Handle("/images/", http.FileServer(http.Dir("web")))
	handler.Handle("/js/", http.FileServer(http.Dir("web")))

	handler.Handle("/pdf/", http.FileServer(http.Dir(config.Config.PDF.Path)))

	handler.HandleFunc("/", defaultHandler)

	authHandler := AuthHandler{handler}
	sessionHandler := seshcookie.NewSessionHandler(&authHandler, SESSIONKEY, nil)

	listenPort := ":"+strconv.Itoa(config.Config.Server.Port)
	if err := http.ListenAndServe(listenPort, sessionHandler); err != nil {
		fmt.Printf("Error in ListenAndServe:", err)
	}

	fmt.Println("Done.")
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Default handler")
	user := control.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
	} else {
		http.Redirect(w, r, "/home", http.StatusFound)
	}
	return
}

//  AuthHandler: handle login/out, then pass other requests onto the basic handler
type AuthHandler struct {
	Handler http.Handler
}

type ReclaimFormData struct {
	Email  string
	Secret string
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if model.Debug >= 1 {
		fmt.Println(" --", model.QueryCount, "database queries so far")
	}
	session := seshcookie.Session.Get(r)
	fmt.Println("\n\nProcessing", r.Method, r.URL.Path)
	fmt.Printf("using session: %#v\n", session)

	// bypass auth for static files
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	fmt.Println("URL segments:", segments)
	first := segments[0]
	fmt.Println("Checking URL segment:", first)
	switch first {
	case "css", "bootstrap", "images", "js", "pdf":
		fmt.Println("Bypassing auth for", first)
		h.Handler.ServeHTTP(w, r)
		return
	}

	switch r.URL.Path {
	case "/users/masq":
		currentUser := control.GetCurrentUser(r)
		if currentUser != nil && currentUser.IsAdmin {
			email := r.FormValue("user")
			user := model.GetUserByEmail(email)
			if user != nil {
				// actually become that user
				fmt.Println("Masquerading as user:", user.Email)
				session["user"] = user.Email
				session["masquerade"] = currentUser.Email
				fmt.Printf("altered session: %#v\n", session)
				http.Redirect(w, r, "/home", http.StatusFound)
				return
			}
		}

	case "/login":
		if r.Method != "POST" {
			http.ServeFile(w, r, "view/login.html")
			return
		}
		err := r.ParseForm()
		if err != nil {
			fmt.Printf("Error '%s' parsing form for %#v\n", err, r)
		}
		email := r.Form.Get("email")
		user := model.GetUserByEmail(email)
		password := r.Form.Get("password")

		if user == nil {
			fmt.Println("Unknown user, redirecting")
			http.Redirect(w, r, "/login", 303)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			fmt.Println("Password incorrect, redirecting", err)
			http.Redirect(w, r, "/login", 303)
			return
		}

		fmt.Printf("Authorized %s\n", user.Name)
		control.PingUser(user.Email)
		session["user"] = user.Email
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	case "/logout":
		if email, ok := session["user"].(string); ok {
			fmt.Printf("Logging out %s\n", email)
		}
		delete(session, "user")
		http.Redirect(w, r, "/login", http.StatusFound)
		return

	case "/account/reclaim/sent":
		http.ServeFile(w, r, "view/account_reclaim_sent.html")
		return

	case "/account/reclaim/done":
		http.ServeFile(w, r, "view/account_reclaim_done.html")
		return

	case "/account/reclaim/incorrect":
		http.ServeFile(w, r, "view/account_reclaim_incorrect.html")
		return

	case "/account/reclaim/nouser":
		http.ServeFile(w, r, "view/account_reclaim_nouser.html")
		return

	case "/account/reclaim":
		err := r.ParseForm()
		if err != nil {
			fmt.Printf("Error '%s' parsing form for %#v\n", err, r)
		}
		email := r.Form.Get("email")
		secret := r.Form.Get("secret")
		user := model.GetUserByEmail(email)
		fmt.Println("Account reclaim: User at", email)

		if r.Method == "POST" {
			if user == nil {
				fmt.Println("Account reclaim: Unknown user:", email)
				http.Redirect(w, r, "/account/reclaim/nouser", http.StatusFound)
				return
			}
			if secret == "" {
				secret := user.GenerateSecret()
				sendSecretEmail(user, secret)
				http.Redirect(w, r, "/account/reclaim/sent", http.StatusFound)
				return
			}

			fmt.Println("Account reclaim: Comparing secret", secret)
			fmt.Println("Account reclaim: Against hash", user.Secret)
			if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(secret)); err == nil {
				password := r.Form.Get("password")
				password2 := r.Form.Get("password2")
				if password != "" && password == password2 {
					fmt.Println("Account reclaim: Setting password")
					hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
					if err == nil {
						user.Password = string(hash)
						user.Secret = ""
						user.Save()
					}
					http.Redirect(w, r, "/account/reclaim/done", http.StatusFound)
					return
				} else {
					fmt.Println("Account reclaim: Redirecting to password form")
					http.Redirect(w, r, "/account/reclaim?email="+email+"&secret="+secret, http.StatusFound)
					return
				}
				return
			} else {
				fmt.Println("Account reclaim: Incorrect:", err)
				http.Redirect(w, r, "/account/reclaim/incorrect", http.StatusFound)
				return
			}
		} else if r.Method == "GET" {
			if secret == "" {
				http.ServeFile(w, r, "view/account_reclaim.html")
				return
			}

			if user == nil {
				fmt.Println("Account reclaim: Unknown user:", email)
				http.Redirect(w, r, "/account/reclaim/nouser", http.StatusFound)
				return
			}

			fmt.Println("Account reclaim: Comparing secret", secret)
			fmt.Println("Account reclaim: Against hash", user.Secret)
			if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(secret)); err == nil {
				fmt.Println("Account reclaim: Showing password form")
				data := ReclaimFormData{
					Email:  email,
					Secret: secret,
				}
				t, _ := template.ParseFiles("view/account_reclaim_set_password.html")
				t.Execute(w, data)
				return
			} else {
				fmt.Println("Account reclaim: Incorrect:", err)
				http.Redirect(w, r, "/account/reclaim/incorrect", http.StatusFound)
				return
			}
		}
	}

	if _, ok := session["user"]; !ok {
		fmt.Printf("Not logged in, redirecting to login")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	fmt.Println("Delivering")
	control.PingCurrentUser(r)
	h.Handler.ServeHTTP(w, r)
}

func sendSecretEmail(user *model.User, secret string) {
	mailConfig := config.Config.Mail

	email := user.Email

	msg := `Subject: Your account at the Character Sheets Translator
Content-Type: text/plain; charset="UTF-8"

This is your password reclaim email for the Dyslexic Character Sheets Translator

To set your password, click here:

http://%s/account/reclaim?email=%s&secret=%s
`
	msg = fmt.Sprintf(msg, control.Hostname, email, secret)
	from := mailConfig.From

	to := []string{user.Email}
	fmt.Println("Sending message to", user.Email, "\n", msg)
	auth := smtp.CRAMMD5Auth(mailConfig.Username, mailConfig.Password)
	err := smtp.SendMail(mailConfig.Hostname, auth, from, to, []byte(msg))
	if err != nil {
		fmt.Println("Error sending mail:", err)
	}
}
