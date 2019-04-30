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
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Base strings for RandStringBytesMaskImprSrc
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)
const (
	appVersion = "1.0.0"
)

var domain string
var redisServer string
var addr string
var port string
var proto string
var path string
var dumpFile string
var urlSize int
var src = rand.NewSource(time.Now().UnixNano())
var pool = cache.New(240*time.Hour, 1*time.Hour)
var indexTmpl = template.Must(template.ParseFiles("templates/index.html"))
var returnTmpl = template.Must(template.ParseFiles("templates/returnPage.html"))
var notFoundTmpl = template.Must(template.ParseFiles("templates/404.html"))
var badRequestTmpl = template.Must(template.ParseFiles("templates/400.html"))
var internalErrorTmpl = template.Must(template.ParseFiles("templates/500.html"))
var okTmpl = template.Must(template.ParseFiles("templates/ok.html"))

func index(w http.ResponseWriter, r *http.Request) {
	indexTmpl.Execute(w, indexTmpl)
}

// get executes the  GET command
func get(key string) (string, bool) {
	value, status := pool.Get(key)
	if status {
		return value.(string), status
	}
	return "", false
}

// set executes the redis SET command
func set(key, suffix string) {
	pool.Set(suffix, key, 0)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	vals := mux.Vars(r)
	key := vals["key"]
	if path != "" {
		key = strings.Replace(key, path, "", 1)
	}
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key = rgx.FindString(key)
	key, status := get(key)
	if status {
		u, _ := url.Parse(key)
		if u.Scheme == "" {
			u.Scheme = "https"
		}
		http.Redirect(w, r, u.String(), http.StatusFound)
	} else {
		w.WriteHeader(http.StatusNotFound)
		notFoundTmpl.Execute(w, notFoundTmpl)
	}
}

func shortner(w http.ResponseWriter, r *http.Request) {
	if govalidator.IsURL(r.FormValue("url")) {
		u, _ := url.Parse(r.FormValue("url"))

		suffix := RandStringBytesMaskImprSrc(urlSize)
		for {
			_, status := get(suffix)
			if status {
				suffix = RandStringBytesMaskImprSrc(urlSize)
			} else {
				break
			}
		}
		var hostSuf string
		if port != "80" && proto == "http" {
			hostSuf = ":" + port + "/"
		} else if port != "443" && proto == "https" {
			hostSuf = ":" + port + "/"
		} else if port == "443" || port == "80" {
			hostSuf = "/"
		}
		set(u.String(), suffix)
		shortend := proto + "://" + domain + hostSuf + path + suffix
		returnTmpl.Execute(w, shortend)

	} else {
		w.WriteHeader(http.StatusBadRequest)
		badRequestTmpl.Execute(w, badRequestTmpl)
	}
}

// RandStringBytesMaskImprSrc Generate random string for URL
func RandStringBytesMaskImprSrc(n int) string {
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

func itemsCount(w http.ResponseWriter, r *http.Request) {
	w.Write(
		[]byte(
			strconv.Itoa(
				pool.ItemCount(),
			),
		),
	)
}

func itemsDump(w http.ResponseWriter, r *http.Request) {
	dumpObj, err := json.Marshal(
		pool.Items(),
	)
	if err != nil {
		log.Fatal("BOOM")
	}
	w.Write(
		[]byte(dumpObj),
	)
}

func itemsFromFile(w http.ResponseWriter, r *http.Request) {
	jsonFile, err := ioutil.ReadFile(dumpFile)
	var dumpObj map[string]cache.Item
	json.Unmarshal([]byte(jsonFile), &dumpObj)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		internalErrorTmpl.Execute(w, "Cannot open file "+dumpFile)
	} else {
		pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
		okTmpl.Execute(w, "Imported "+strconv.Itoa(len(dumpObj))+" items to the DB")
	}
}

func itemsFromPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dumpObj map[string]cache.Item
	err := decoder.Decode(&dumpObj)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		internalErrorTmpl.Execute(w, "Cannot parse JSON")
	} else {
		pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
		okTmpl.Execute(w, "Imported "+strconv.Itoa(len(dumpObj))+" items to the DB")
	}
}

func itemsDumpToFile(w http.ResponseWriter, r *http.Request) {
	dumpObj, _ := json.Marshal(
		pool.Items(),
	)
	err := ioutil.WriteFile(dumpFile, dumpObj, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		internalErrorTmpl.Execute(w, "Failed to open json file")
	} else {
		okTmpl.Execute(w, "Dump writen to: "+dumpFile)
	}
}

func main() {
	flag.StringVar(&addr, "addr", "localhost", "Address to listen for connections")
	flag.StringVar(&domain, "domain", "localhost", "Domain to write to the URLs")
	flag.StringVar(&dumpFile, "dump", "urls.json", "Path to the file to dump the kv db")
	flag.StringVar(&path, "path", "", "Path to the base URL (https://localhost/PATH/... remember to append a / at the end")
	flag.IntVar(&port, "port", 8080, "Port to listen for connections")
	flag.StringVar(&proto, "proto", "https", "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
	flag.IntVar(&urlSize, "urlsize", 10, "Define the size of the shortened String, default 10")
	version := flag.Bool("v", false, "prints current version")
	flag.Parse()
	if *version {
		fmt.Printf("%s", appVersion)
		os.Exit(0)
	}

	if port > 65535 || port < 1 {

	}
	if path != "" && !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	ip := net.ParseIP(addr)
	if ip != nil {
		listenAddr = ip.String() + ":" + strconv.Itoa(port)
	} else {
		if govalidator.IsDNSName(addr) {
			listenAddr = addr + ":" + strconv.Itoa(port)
		} else {
			log.Fatalln("Invalid ip address")
		}
	}

	if !govalidator.IsDNSName(domain) {
		log.Fatalln("Invalid domain address")
	}

	r := mux.NewRouter()

	r.HandleFunc("/", index).Methods("GET")
	r.HandleFunc("/", shortner).Methods("POST")
	r.HandleFunc("/{key}", redirect).Methods("GET")
	r.HandleFunc("/v1/count", itemsCount).Methods("GET")
	r.HandleFunc("/v1/dump", itemsDump).Methods("GET")
	r.HandleFunc("/v1/dumpToFile", itemsDumpToFile).Methods("GET")
	r.HandleFunc("/v1/fromFile", itemsFromFile).Methods("GET")
	r.HandleFunc("/v1/fromPost", itemsFromPost).Methods("POST")
	log.Printf("Domain: %s, URL Proto: %s\n", domain, proto)
	log.Fatal(http.ListenAndServe(listenAddr, handlers.CombinedLoggingHandler(os.Stdout, r)))
}
