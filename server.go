package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/smallnest/rpcx-ui/service"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore(securecookie.GenerateRandomKey(32))

var templates map[string]*template.Template
var loginBytes, _ = ioutil.ReadFile("./templates/login.html")
var loginTemp, _ = template.New("login").Parse(string(loginBytes))

// Load templates on program initialisation
func init() {
	//https: //elithrar.github.io/article/approximating-html-template-inheritance/

	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	templatesDir := "./templates/"

	//pages to show indeed
	bases, err := filepath.Glob(templatesDir + "bases/*.html")
	if err != nil {
		log.Fatal(err)
	}

	//widgts, header, footer, sidebar, etc.
	includes, err := filepath.Glob(templatesDir + "includes/*.html")
	if err != nil {
		log.Fatal(err)
	}

	// Generate our templates map from our bases/ and includes/ directories
	for _, base := range bases {
		files := append(includes, base)
		templates[filepath.Base(base)] = template.Must(template.ParseFiles(files...))
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	// Ensure the template exists in the map.
	tmpl, ok := templates[name]
	if !ok {
		return fmt.Errorf("The template %s does not exist.", name)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, name, data)
}

func main() {
	flag.Parse()
	service.LoadConfig()

	http.HandleFunc("/logout", func(rw http.ResponseWriter, req *http.Request) {
		session, _ := store.Get(req, "gosessionid")
		session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
		session.Save(req, rw)
		http.Redirect(rw, req, "/", http.StatusFound)
	})

	http.HandleFunc("/", authWrapper(recoverWrapper(indexHandler)))

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {

		var errMsg = ""
		if r.Method == http.MethodPost {
			username := r.FormValue("username")
			password := r.FormValue("password")

			if username == service.ServerConfig.User && password == service.ServerConfig.Password {
				session, _ := store.Get(r, "gosessionid")
				session.Values["userLogin"] = username
				session.Save(r, w)
				http.Redirect(w, r, "/services", http.StatusFound)
				return
			}

			errMsg = "username or password is not correct"
			loginTemp.ExecuteTemplate(w, "login", errMsg)
		}

		if r.Method == http.MethodGet {
			loginTemp.ExecuteTemplate(w, "login", nil)
		}
	})

	http.HandleFunc("/services", authWrapper(recoverWrapper(servicesHandler)))
	http.HandleFunc("/s/deactivate/", authWrapper(recoverWrapper(deactivateHandler)))
	http.HandleFunc("/s/activate/", authWrapper(recoverWrapper(activateHandler)))
	http.HandleFunc("/s/m/", authWrapper(recoverWrapper(modifyHandler)))
	http.HandleFunc("/registry", authWrapper(recoverWrapper(registryHandler)))

	fs := http.FileServer(http.Dir("web"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.ListenAndServe(service.ServerConfig.Host+":"+strconv.Itoa(service.ServerConfig.Port), nil)
}

func authWrapper(h func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "gosessionid")
		username := session.Values["userLogin"]
		if username != nil {
			h(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	}
}

func recoverWrapper(h func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if re := recover(); re != nil {
				var err error
				fmt.Println("Recovered in registryHandler", re)
				switch t := re.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = errors.New("Unknown error")
				}
				w.WriteHeader(http.StatusOK)
				renderTemplate(w, "error.html", err.Error())
			}
		}()
		h(w, r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/services", http.StatusFound)
}

func servicesHandler(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	data["services"] = service.Reg.FetchServices()
	renderTemplate(w, r.URL.Path[1:]+".html", data)
}

func deactivateHandler(w http.ResponseWriter, r *http.Request) {
	i := strings.LastIndex(r.URL.Path, "/")
	base64ID := r.URL.Path[i+1:]

	if b, err := base64.StdEncoding.DecodeString(base64ID); err == nil {
		s := string(b)
		j := strings.Index(s, "@")
		name := s[0:j]
		address := s[j+1:]
		service.Reg.DeactivateService(name, address)
	}
	http.Redirect(w, r, "/services", http.StatusFound)
}

func activateHandler(w http.ResponseWriter, r *http.Request) {
	i := strings.LastIndex(r.URL.Path, "/")
	base64ID := r.URL.Path[i+1:]

	if b, err := base64.StdEncoding.DecodeString(base64ID); err == nil {
		s := string(b)
		j := strings.Index(s, "@")
		name := s[0:j]
		address := s[j+1:]
		service.Reg.ActivateService(name, address)
	}

	http.Redirect(w, r, "/services", http.StatusFound)
}

func modifyHandler(w http.ResponseWriter, r *http.Request) {
	metadata := r.URL.Query()

	i := strings.LastIndex(r.URL.Path, "/")
	base64ID := r.URL.Path[i+1:]

	if b, err := base64.StdEncoding.DecodeString(base64ID); err == nil {
		s := string(b)
		j := strings.Index(s, "@")
		name := s[0:j]
		address := s[j+1:]
		service.Reg.UpdateMetadata(name, address, metadata.Encode())
	}

	http.Redirect(w, r, "/services", http.StatusFound)
}

func registryHandler(w http.ResponseWriter, r *http.Request) {
	oldConfig := service.ServerConfig
	defer func() {
		if re := recover(); re != nil {
			bytes, err := json.MarshalIndent(&oldConfig, "", "\t")
			if err == nil {
				err = ioutil.WriteFile("./config.json", bytes, 0644)
				service.LoadConfig()
			}

			panic(re)
		}
	}()

	if r.Method == "POST" {
		registryType := r.FormValue("registry_type")
		registryURL := r.FormValue("registry_url")
		basePath := r.FormValue("base_path")

		service.ServerConfig.RegistryType = registryType
		service.ServerConfig.RegistryURL = registryURL
		service.ServerConfig.ServiceBaseURL = basePath

		bytes, err := json.MarshalIndent(&service.ServerConfig, "", "\t")
		if err == nil {
			err = ioutil.WriteFile("./config.json", bytes, 0644)
			service.LoadConfig()
		}
	}

	renderTemplate(w, r.URL.Path[1:]+".html", service.ServerConfig)
}
