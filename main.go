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

type config struct {
	addr       string
	domain     string
	dumpFile   string
	path       string
	proto      string
	hostSuf    string
	listenAddr string
	port       int
	urlSize    int
	version    bool
	templates  *template.Template
}

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
	appVersion    = "1.2.1"
)

var (
	cfg  config
	src  = rand.NewSource(time.Now().UnixNano())
	pool = cache.New(240*time.Hour, 1*time.Hour)
)

func init() {
	flag.StringVar(&cfg.addr, "addr", "localhost", "Address to listen for connections")
	flag.StringVar(&cfg.domain, "domain", "localhost", "Domain to write to the URLs")
	flag.StringVar(&cfg.dumpFile, "dump", "urls.json", "Path to the file to dump the kv db")
	flag.StringVar(&cfg.path, "path", "", "Path to the base URL (https://localhost/PATH/... remember to append a / at the end")
	flag.StringVar(&cfg.proto, "proto", "https", "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
	flag.IntVar(&cfg.port, "port", 8080, "Port to listen for connections")
	flag.IntVar(&cfg.urlSize, "urlsize", 10, "Define the size of the shortened String, default 10")
	flag.BoolVar(&cfg.version, "v", false, "prints current version")
	flag.Parse()

	if cfg.version {
		log.SetFlags(0)
		log.Println(appVersion)
		os.Exit(0)
	}

	if cfg.port > 65535 || cfg.port < 1 {
		log.Fatalln("Invalid port number")
	}
	if cfg.path != "" && !strings.HasSuffix(cfg.path, "/") {
		cfg.path = cfg.path + "/"
	}

	if cfg.port != 80 && cfg.proto == "http" {
		cfg.hostSuf = ":" + strconv.Itoa(cfg.port) + "/"
	} else if cfg.port != 443 && cfg.proto == "https" {
		cfg.hostSuf = ":" + strconv.Itoa(cfg.port) + "/"
	} else if cfg.port == 443 || cfg.port == 80 {
		cfg.hostSuf = "/"
	}

	ip := net.ParseIP(cfg.addr)
	if ip != nil {
		cfg.listenAddr = ip.String() + ":" + strconv.Itoa(cfg.port)
	} else {
		if govalidator.IsDNSName(cfg.addr) {
			cfg.listenAddr = cfg.addr + ":" + strconv.Itoa(cfg.port)
		} else {
			log.Fatalln("Invalid ip address")
		}
	}

	if !govalidator.IsDNSName(cfg.domain) {
		log.Fatalln("Invalid domain address")
	}

	cfg.templates = template.Must(template.ParseFiles("templates/response.html"))
}

func (c config) index(w http.ResponseWriter, r *http.Request) {
	b := body{
		HasForm: true,
		Line1:   "Welcome to Short, the simple URL shortener,",
		Line2:   "Type an URL below to shorten it",
	}
	c.templates.Execute(w, b)
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
func (c config) redirect(w http.ResponseWriter, r *http.Request) {
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
	if c.path != "" {
		key = strings.Replace(key, c.path, "", 1)
	}
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key = rgx.FindString(key)
	key, status := get(key)
	if !status {
		w.WriteHeader(http.StatusNotFound)
		c.templates.Execute(w, b)
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
func (c config) shortner(w http.ResponseWriter, r *http.Request) {
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
		c.templates.Execute(w, b)
		return
	}
	u, _ := url.Parse(r.FormValue("url"))
	suffix := randStringBytesMaskImprSrc(c.urlSize)

	for {
		_, status := get(suffix)
		if !status {
			break
		}
		suffix = randStringBytesMaskImprSrc(c.urlSize)
	}
	set(u.String(), suffix)
	shortend := c.proto + "://" + c.domain + c.hostSuf + c.path + suffix
	b := body{
		IsLink: true,
		Line1:  shortend,
	}
	c.templates.Execute(w, b)
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
func (c config) internalError(w http.ResponseWriter, msg string, err error) {
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
	c.templates.Execute(w, b)
}

// itemsCount returns the number of kv pairs on the in meomry database
func (c config) itemsCount(w http.ResponseWriter, r *http.Request) {
	w.Write(
		[]byte(
			strconv.Itoa(
				pool.ItemCount(),
			),
		),
	)
}

// itemsDump returns a json with all the kv pairs on the in memory database
func (c config) itemsDump(w http.ResponseWriter, r *http.Request) {
	dumpObj, err := json.Marshal(
		pool.Items(),
	)
	if err != nil {
		c.internalError(w, "Unable to dump key value db: ", err)
	}
	w.Write(
		[]byte(dumpObj),
	)
}

// itemsFromFile loads kv pairs from the dumpFile json to the in memory database
func (c config) itemsFromFile(w http.ResponseWriter, r *http.Request) {
	jsonFile, err := ioutil.ReadFile(c.dumpFile)
	var dumpObj map[string]cache.Item
	json.Unmarshal([]byte(jsonFile), &dumpObj)
	if err != nil {
		c.internalError(w, "Cannot open file "+c.dumpFile+": ", err)
		return
	}
	pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
	b := body{
		Line1: "Imported " + strconv.Itoa(len(dumpObj)) + " items to the DB",
	}
	c.templates.Execute(w, b)
}

// itemsFromPost loads kv pairs from a json POST to the in memory database
func (c config) itemsFromPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dumpObj map[string]cache.Item
	err := decoder.Decode(&dumpObj)
	if err != nil {
		c.internalError(w, "Cannot parse JSON: ", err)
		return
	}
	pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
	b := body{
		Line1: "Imported " + strconv.Itoa(len(dumpObj)) + " items to the DB",
	}
	c.templates.Execute(w, b)
}

// itemsDumpToFile dumps the kv pairs from the in memory database to the dumpFile
func (c config) itemsDumpToFile(w http.ResponseWriter, r *http.Request) {
	dumpObj, _ := json.Marshal(
		pool.Items(),
	)
	err := ioutil.WriteFile(c.dumpFile, dumpObj, 0644)
	if err != nil {
		c.internalError(w, "Failed to open json file: ", err)
		return
	}
	b := body{
		Line1: "Imported " + "Dump writen to: " + c.dumpFile,
	}
	c.templates.Execute(w, b)
}

func main() {

	r := mux.NewRouter()

	r.HandleFunc("/", cfg.index).Methods("GET")
	r.HandleFunc("/", cfg.shortner).Methods("POST")
	r.HandleFunc("/{key}", cfg.redirect).Methods("GET")
	r.HandleFunc("/v1/dumpToFile", cfg.itemsDumpToFile).Methods("GET")
	r.HandleFunc("/v1/fromFile", cfg.itemsFromFile).Methods("GET")
	r.HandleFunc("/v1/count", cfg.itemsCount).Methods("GET")
	r.HandleFunc("/v1/dump", cfg.itemsDump).Methods("GET")
	r.HandleFunc("/v1/fromPost", cfg.itemsFromPost).Methods("POST")

	log.Printf("Domain: %s, URL Proto: %s, Listen Address: %s\n", cfg.domain, cfg.proto, cfg.listenAddr)
	log.Fatal(http.ListenAndServe(cfg.listenAddr, handlers.CombinedLoggingHandler(os.Stdout, r)))
}
