package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Base strings for randStringBytesMaskImprSrc
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)
const appVersion = "1.1.0"

var src = rand.NewSource(time.Now().UnixNano())
var pool = cache.New(240*time.Hour, 1*time.Hour)
var t = template.Must(template.ParseFiles("templates/response.html"))

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

func index(w http.ResponseWriter, r *http.Request) {
	b := body{
		HasForm: true,
		Line1:   "Welcome to Short, the simple URL shortener,",
		Line2:   "Type an URL below to shorten it",
	}
	t.Execute(w, b)
}

// get executes the  GET command
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

// redirect reads the key from the requests url (GET /key) searches the
// kv database for it and if found redirects the user to value, if not
// found return a 404.
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	vals := mux.Vars(r)
	key := vals["key"]
	b := body{
		FullHeader: true,
		IsGhost:    true,
		HasForm:    true,
		H1:         "404",
		H3:         "page not found",
		Line1:      "Boo, looks like this ghost stole this page!",
		Line2:      "But you can type an URL below to shorten it",
	}
	if path != "" {
		key = strings.Replace(key, path, "", 1)
	}
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key = rgx.FindString(key)
	key, status := get(key)
	if !status {
		w.WriteHeader(http.StatusNotFound)
		t.Execute(w, b)
		return
	}
	u, _ := url.Parse(key)
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// shortner reads url from a POST request, validates the url, generate a
// random suffix string of urlSize size, checks if the suffix string is
// unique on the kv database and if not unique regenerates it and checks again,
// then if writes the kv pair suffix, url to the database and return the
// shortened url to the user
func shortner(w http.ResponseWriter, r *http.Request, proto, domain, hostSuf, path string, urlSize int) {
	if !govalidator.IsURL(r.FormValue("url")) {
		b := body{
			FullHeader: true,
			IsGhost:    true,
			HasForm:    true,
			H1:         "400",
			H3:         "bad request",
			Line1:      "Boo, looks like this ghost stole this page!",
			Line2:      "But you can type an URL below to shorten it",
		}
		w.WriteHeader(http.StatusBadRequest)
		t.Execute(w, b)
		return
	}
	u, _ := url.Parse(r.FormValue("url"))
	suffix := randStringBytesMaskImprSrc(urlSize)

	for {
		_, status := get(suffix)
		if !status {
			break
		}
		suffix = randStringBytesMaskImprSrc(urlSize)
	}
	set(u.String(), suffix)
	shortend := proto + "://" + domain + hostSuf + path + suffix
	b := body{
		IsLink: true,
		Line1:  shortend,
	}
	t.Execute(w, b)
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

// internalError receives a http.ResponseWriter, msg and error and
// return a internal error page with http code 500 to the user
func internalError(w http.ResponseWriter, msg string, err error) {
	b := body{
		FullHeader: true,
		IsGhost:    true,
		HasForm:    true,
		H1:         "500",
		H3:         "internal erver error",
		Line1:      "Boo, the ghost is broken :(",
		Line2:      "His last words where: " + err.Error(),
	}
	log.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
	t.Execute(w, b)
}

// itemsCount returns the number of kv pairs on the in meomry database
func itemsCount(w http.ResponseWriter, r *http.Request) {
	w.Write(
		[]byte(
			strconv.Itoa(
				pool.ItemCount(),
			),
		),
	)
}

// itemsDump returns a json with all the kv pairs on the in memory database
func itemsDump(w http.ResponseWriter, r *http.Request) {
	dumpObj, err := json.Marshal(
		pool.Items(),
	)
	if err != nil {
		internalError(w, "Unable to dump key value db: ", err)
	}
	w.Write(
		[]byte(dumpObj),
	)
}

// itemsFromFile loads kv pairs from the dumpFile json to the in memory database
func itemsFromFile(w http.ResponseWriter, r *http.Request, dumpFile string) {
	jsonFile, err := ioutil.ReadFile(dumpFile)
	var dumpObj map[string]cache.Item
	json.Unmarshal([]byte(jsonFile), &dumpObj)
	if err != nil {
		internalError(w, "Cannot open file "+dumpFile+": ", err)
		return
	}
	pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
	b := body{
		Line1: "Imported " + strconv.Itoa(len(dumpObj)) + " items to the DB",
	}
	t.Execute(w, b)
}

// itemsFromPost loads kv pairs from a json POST to the in memory database
func itemsFromPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dumpObj map[string]cache.Item
	err := decoder.Decode(&dumpObj)
	if err != nil {
		internalError(w, "Cannot parse JSON: ", err)
		return
	}
	pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
	b := body{
		Line1: "Imported " + strconv.Itoa(len(dumpObj)) + " items to the DB",
	}
	t.Execute(w, b)
}

// itemsDumpToFile dumps the kv pairs from the in memory database to the dumpFile
func itemsDumpToFile(w http.ResponseWriter, r *http.Request, dumpFile string) {
	dumpObj, _ := json.Marshal(
		pool.Items(),
	)
	err := ioutil.WriteFile(dumpFile, dumpObj, 0644)
	if err != nil {
		internalError(w, "Failed to open json file: ", err)
		return
	}
	b := body{
		Line1: "Imported " + "Dump writen to: " + dumpFile,
	}
	t.Execute(w, b)

}

func main() {
	var hostSuf string
	var listenAddr string

	addr := flag.String("addr", "localhost", "Address to listen for connections")
	domain := flag.String("domain", "localhost", "Domain to write to the URLs")
	dumpFile := flag.String("dump", "urls.json", "Path to the file to dump the kv db")
	path := flag.String("path", "", "Path to the base URL (https://localhost/PATH/... remember to append a / at the end")
	port := flag.Int("port", 8080, "Port to listen for connections")
	proto := flag.String("proto", "https", "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
	urlSize := flag.Int("urlsize", 10, "Define the size of the shortened String, default 10")
	version := flag.Bool("v", false, "prints current version")
	flag.Parse()
	if *version {
		log.SetFlags(0)
		log.Println(appVersion)
		os.Exit(0)
	}

	if *port > 65535 || *port < 1 {
		log.Fatalln("Invalid port number")
	}
	if *path != "" && !strings.HasSuffix(*path, "/") {
		*path = *path + "/"
	}
	if *port != 80 && *proto == "http" {
		hostSuf = ":" + strconv.Itoa(*port) + "/"
	} else if *port != 443 && *proto == "https" {
		hostSuf = ":" + strconv.Itoa(*port) + "/"
	} else if *port == 443 || *port == 80 {
		hostSuf = "/"
	}
	ip := net.ParseIP(*addr)
	if ip != nil {
		listenAddr = ip.String() + ":" + strconv.Itoa(*port)
	} else {
		if govalidator.IsDNSName(*addr) {
			listenAddr = *addr + ":" + strconv.Itoa(*port)
		} else {
			log.Fatalln("Invalid ip address")
		}
	}

	if !govalidator.IsDNSName(*domain) {
		log.Fatalln("Invalid domain address")
	}

	r := mux.NewRouter()

	r.HandleFunc("/", index).Methods("GET")

	r.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			shortner(w, r, *proto, *domain, hostSuf, *path, *urlSize)
		}).Methods("POST")

	r.HandleFunc("/{key}",
		func(w http.ResponseWriter, r *http.Request) {
			redirect(w, r, *path)
		}).Methods("GET")

	r.HandleFunc("/v1/dumpToFile",
		func(w http.ResponseWriter, r *http.Request) {
			itemsDumpToFile(w, r, *dumpFile)
		}).Methods("GET")

	r.HandleFunc("/v1/fromFile",
		func(w http.ResponseWriter, r *http.Request) {
			itemsFromFile(w, r, *dumpFile)
		}).Methods("GET")

	r.HandleFunc("/v1/count", itemsCount).Methods("GET")
	r.HandleFunc("/v1/dump", itemsDump).Methods("GET")
	r.HandleFunc("/v1/fromPost", itemsFromPost).Methods("POST")

	log.Printf("Domain: %s, URL Proto: %s, Listen Address: %s\n", *domain, *proto, *addr)
	log.Fatal(http.ListenAndServe(listenAddr, handlers.CombinedLoggingHandler(os.Stdout, r)))
}
