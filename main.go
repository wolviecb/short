package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

type body struct {
	FullHeader bool
	IsGhost    bool
	HasForm    bool
	IsLink     bool
	H1         string
	H3         string
	Line1      string
	Line2      string
}

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Base strings for randStringBytesMaskImprSrc
	letterIdxBits = 6                                                                // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1                                             // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits                                               // # of letter indices fitting in 63 bits
	appVersion    = "1.3.0"
)

var (
	// tiny entropy pool
	src = rand.NewSource(time.Now().UnixNano())
	// KV memory DB
	pool *cache.Cache
	// Error codes
	errBadRequest = fmt.Errorf("Bad Request")
	errNotFound   = fmt.Errorf("Not Found")
)

// get executes the GET command
func get(key string) (string, bool) {
	value, status := pool.Get(key)
	if !status {
		return "", status
	}
	return value.(string), status
}

// set executes the redis SET command
func set(key, suffix string) {
	pool.Set(suffix, key, 0)
}

// redirect receives a key searches the kv database for it and if
// found returns the value, or a error if not found
func redirect(k string) (string, error) {
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key := rgx.FindString(k)
	key, status := get(key)
	if !status {
		return "", errNotFound
	}
	u, _ := url.Parse(key)
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u.String(), nil
}

// shortener receive a url, validates the url, generate a random suffix string
// of urlSize size, checks if the suffix string is ensure on the kv database
// and then writes the kv pair (suffix, url) to the database, returning the suffix
func shortener(u string, s int) (string, error) {
	var su string
	if !govalidator.IsURL(u) {
		return su, errBadRequest
	}
	pu, _ := url.Parse(u)

	for {
		su = randStringBytesMaskImprSrc(s)
		_, status := get(su)
		if !status {
			break
		}
	}

	set(pu.String(), su)
	return su, nil
}

// randStringBytesMaskImprSrc Generate random string of n size
func randStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

func internalError(msg string, err error) body {
	log.Println(err)
	return body{
		FullHeader: true,
		IsGhost:    true,
		HasForm:    true,
		H1:         "500",
		H3:         msg,
		Line1:      "Boo, the ghost is broken :(",
		Line2:      "His last words where: " + err.Error(),
	}
}

// loadFromFile loads kv pairs from the dumpFile json to the in memory database
func loadFromFile(file string, e, c int) (int, error) {
	dumpObj := make(map[string]cache.Item)
	jsonFile, err := ioutil.ReadFile(file)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal([]byte(jsonFile), &dumpObj)
	if err != nil {
		return 0, err
	}

	pool = cache.NewFrom(time.Duration(e)*time.Hour, time.Duration(c)*time.Hour, dumpObj)
	return len(dumpObj), err
}

// itemsFromPost loads kv pairs from a json POST to the in memory database
func loadFromJSON(j []byte, e, c int) (int, error) {
	dumpObj := make(map[string]cache.Item)
	err := json.Unmarshal(j, &dumpObj)
	if err != nil {
		return 0, err
	}

	pool = cache.NewFrom(time.Duration(e)*time.Hour, time.Duration(c)*time.Hour, dumpObj)
	return len(dumpObj), nil
}

// dumpDbToFile dumps the kv pairs from the in memory database to file
func dumpDbTOFile(file string) (int, error) {
	i := pool.Items()
	dumpObj, _ := json.Marshal(i)
	return len(i), ioutil.WriteFile(file, dumpObj, 0644)
}

func main() {
	var (
		addr       = flag.String("addr", "localhost", "Address to listen for connections")
		domain     = flag.String("domain", "localhost", "Domain to write to the URLs")
		dumpFile   = flag.String("dump", "urls.json", "Path to the file to dump the kv db")
		path       = flag.String("path", "", "Path to the base URL (https://localhost/PATH/...")
		proto      = flag.String("proto", "https", "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
		port       = flag.Int("port", 8080, "Port to listen for connections")
		urlSize    = flag.Int("urlsize", 10, "Define the size of the shortened String, default 10")
		exp        = flag.Int("exp", 240, "Default expiration time in hours, default 240")
		cleanup    = flag.Int("cleanup", 1, "Cleanup interval in hours, default 1")
		version    = flag.Bool("v", false, "prints current version")
		listenAddr string
	)

	flag.Parse()

	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	if *port > 65535 || *port < 1 {
		log.Fatalln("Invalid port number")
	}
	if *path != "" && !strings.HasSuffix(*path, "/") {
		*path = *path + "/"
	}

	ip := net.ParseIP(*addr)
	if ip != nil {
		listenAddr = fmt.Sprintf("%s:%v", ip.String(), *port)
	} else {
		if govalidator.IsDNSName(*addr) {
			listenAddr = fmt.Sprintf("%s:%v", *addr, *port)
		} else {
			log.Fatalln("Invalid ip address")
		}
	}

	if !govalidator.IsDNSName(*domain) {
		log.Fatalln("Invalid domain address")
	}

	pool = cache.New(time.Duration(*exp)*time.Hour, time.Duration(*cleanup)*time.Hour)
	t := template.Must(template.ParseFiles("templates/response.html"))
	r := mux.NewRouter()

	// Index
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Execute(w, body{
			HasForm: true,
			Line1:   "Welcome to Short, the simple URL shortener,",
			Line2:   "Type an URL below to shorten it",
		})
	}).Methods("GET")

	// URL Shortener
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		suf, err := shortener(r.FormValue("url"), *urlSize)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.Execute(w, body{
				FullHeader: true,
				IsGhost:    true,
				HasForm:    true,
				H1:         "400",
				H3:         err.Error(),
				Line1:      "Boo, looks like this ghost stole this page!",
				Line2:      "But you can type an URL below to shorten it",
			})
			return
		}
		ru, _ := url.Parse(fmt.Sprintf("%s://%s:%v/%s%s", *proto, *domain, *port, *path, suf))
		t.Execute(w, body{
			IsLink: true,
			Line1:  ru.String(),
		})
	}).Methods("POST")

	// URL Redirect
	r.HandleFunc("/{key}", func(w http.ResponseWriter, r *http.Request) {
		vals := mux.Vars(r)
		key := vals["key"]
		if *path != "" {
			key = strings.Replace(key, *path, "", 1)
		}
		u, err := redirect(key)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			t.Execute(w, body{
				FullHeader: true,
				IsGhost:    true,
				HasForm:    true,
				H1:         "404",
				H3:         err.Error(),
				Line1:      "Boo, looks like this ghost stole this page!",
				Line2:      "But you can type an URL below to shorten it",
			})
			return
		}
		http.Redirect(w, r, u, http.StatusFound)
	}).Methods("GET")

	// Dump DB to file
	r.HandleFunc("/v1/toFile", func(w http.ResponseWriter, r *http.Request) {
		i, err := dumpDbTOFile(*dumpFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			t.Execute(w, internalError("Failed to dump kv DB to file", err))
			return
		}
		t.Execute(w, body{Line1: fmt.Sprintf("Exported %v items to %v", i, dumpFile)})
	}).Methods("GET")

	// Read DB from file
	r.HandleFunc("/v1/fromFile", func(w http.ResponseWriter, r *http.Request) {
		i, err := loadFromFile(*dumpFile, *exp, *cleanup)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			t.Execute(w, internalError("Error loading DB from file", err))
			return
		}
		t.Execute(w, body{Line1: fmt.Sprintf("Imported %v items to the DB", i)})
	}).Methods("GET")

	// Count items on DB
	r.HandleFunc("/v1/count", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%v", pool.ItemCount())
	}).Methods("GET")

	r.HandleFunc("/v1/dump", func(w http.ResponseWriter, r *http.Request) {
		dumpObj, err := json.Marshal(pool.Items())
		if err != nil {
			t.Execute(w, internalError("Unable to dump key value db: ", err))
			return
		}
		fmt.Fprintf(w, "%s", dumpObj)
	}).Methods("GET")

	// Loads DB from json POST
	r.HandleFunc("/v1/fromPost", func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Execute(w, internalError("Unable to dump key value db: ", err))
			return
		}
		i, err := loadFromJSON(b, *exp, *cleanup)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			t.Execute(w, internalError("Error loading DB", err))
			return
		}
		t.Execute(w, body{Line1: fmt.Sprintf("Imported %v items to the DB", i)})
	}).Methods("POST")

	log.Printf("Domain: %s, URL Proto: %s, Listen Address: %s\n", *domain, *proto, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, handlers.CombinedLoggingHandler(os.Stdout, r)))
}
