package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	appVersion = "0.2.0"
)

const indexPage = `
<!DOCTYPE html>
<html lang=en>
  <head>
    <title>Short: the simple url shortener</title>
    <style>
    form{
			position:fixed;
			top:30%;
			left:40%;
			width:500px;
			font-family:georgia,garamond,serif;
			font-size:16px;

		}
    </style>
  </head>
<body>
  <form action="/" method="POST">
			<label for="url">
			Please type the url</label>
				<br>
        <input id="url" type="text" name="url"/>
				<input type="submit" name="Submit" value="Submit"/>
  </form>
</body>
</html>
`

const returnPage = `
<!DOCTYPE html>
<html lang=en>
  <head>
    <title>Short: the simple url shortner</title>
    <style>
    .center {
			padding: 70px 0;
			border: none;
			border-color: transparent;
			text-align: center;
			font-family:georgia,garamond,serif;
			font-size:16px;
		}
    </style>
  </head>
<body>
	<div class="center">
		URL Shortened to <a href="%s">%s</a>
	</div>
</body>
</html>
`

var domain string
var redisServer string
var addr string
var port string
var proto string
var path string
var dumpFile string
var src = rand.NewSource(time.Now().UnixNano())
var pool = cache.New(240*time.Hour, 1*time.Hour)

func index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(indexPage))
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
func set(url, suffix string) {
	pool.Set(suffix, url, 0)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	vals := mux.Vars(r)
	val := vals["key"]
	if path != "" {
		val = strings.Replace(val, path, "", 1)
	}
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key := rgx.FindString(val)
	url, status := get(key)
	if status {
		http.Redirect(w, r, url, http.StatusFound)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("URL don't exist"))
	}
}

func shortner(w http.ResponseWriter, r *http.Request) {
	u, err := url.ParseRequestURI(r.FormValue("url"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad URL"))
	} else {
		suffix := RandStringBytesMaskImprSrc(10)
		for {
			_, status := get(suffix)
			if status {
				suffix = RandStringBytesMaskImprSrc(10)
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
		output := fmt.Sprintf(returnPage, shortend, shortend)
		w.Write([]byte(output))
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
		w.Write(
			[]byte("Cannot open file " + dumpFile),
		)
	} else {
		pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
		w.Write(
			[]byte("OK"),
		)
	}
}

func itemsFromPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dumpObj map[string]cache.Item
	err := decoder.Decode(&dumpObj)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(
			[]byte("Cannot parse JSON"),
		)
	} else {
		pool = cache.NewFrom(240*time.Hour, 1*time.Hour, dumpObj)
		w.Write(
			[]byte("OK"),
		)
	}
}

func itemsDumpToFile(w http.ResponseWriter, r *http.Request) {
	dumpObj, _ := json.Marshal(
		pool.Items(),
	)
	err := ioutil.WriteFile(dumpFile, dumpObj, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(
			[]byte("Failed to open json file"),
		)
	} else {
		w.Write([]byte("Dump writen to: " + dumpFile))
	}
}

func main() {
	flag.StringVar(&domain, "domain", "localhost", "Domain to write to the URLs")
	flag.StringVar(&addr, "addr", "localhost", "Address to listen for connections")
	flag.StringVar(&port, "port", "8080", "Port to listen for connections")
	flag.StringVar(&path, "path", "", "Path to the base URL (https://localhost/PATH/... remember to append a / at the end")
	flag.StringVar(&dumpFile, "dump", "urls.json", "Path to the file to dump the kv db")
	flag.StringVar(&proto, "proto", "https", "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
	version := flag.Bool("v", false, "prints current version")
	flag.Parse()
	if *version {
		fmt.Printf("%s", appVersion)
		os.Exit(0)
	}

	if path != "" && !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	listenAddr := addr + ":" + port

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
