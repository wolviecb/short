package shortie

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/patrickmn/go-cache"
	"github.com/valyala/fasthttp"
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
)

var (
	// Src tiny entropy Pool
	Src = rand.NewSource(time.Now().UnixNano())
	// Pool KV memory DB
	Pool *cache.Cache
	// URLSize is the default size of the shortened URL
	URLSize int = 10
	// Port is the listening port address
	Port int = 8080
	// Exp is the expiration time of the shortened URL
	Exp time.Duration = 240 * time.Hour
	// Cleanup is the clean up time interval
	Cleanup time.Duration = 1 * time.Hour
	// Proto is the shortened URL protocol
	Proto string = "https"
	// Domain is the shortened URL domain
	Domain string = "localhost"
	// Path is the shortened URL prefix path
	Path string = ""
	// DumpFile is the file to dump URL data
	DumpFile string = "urls.json"
	// Error Definitions
	ErrBadRequest = fmt.Errorf("bad request")
	ErrNotFound   = fmt.Errorf("not found")
)

// get executes the GET command
func get(key string) (string, bool) {
	value, status := Pool.Get(key)
	if !status {
		return "", status
	}
	return value.(string), status
}

// set executes the redis SET command
func set(key, suffix string) {
	Pool.Set(suffix, key, 0)
}

// randStringBytesMaskImprSrc Generate random string of n size
func randStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, Src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = Src.Int63(), letterIdxMax
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

// redirect receives a key searches the kv database for it and if
// found returns the value, or a error if not found
func redirect(k string) (string, error) {
	rgx, _ := regexp.Compile("[a-zA-Z0-9]+")
	key := rgx.FindString(k)
	key, status := get(key)
	if !status {
		return "", ErrNotFound
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
	if !govalidator.IsURL(string(u)) {
		return su, ErrBadRequest
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

// dumpDbToFile dumps the kv pairs from the in memory database to file
func dumpDbTOFile(f *os.File) (int, error) {
	i := Pool.Items()
	dumpObj, _ := json.Marshal(i)
	if _, err := f.Write(dumpObj); err != nil {
		return len(i), err
	}
	return len(i), nil
}

// loadFromFile loads kv pairs from the dumpFile json to the in memory database
func loadFromFile() (int, error) {
	dumpObj := make(map[string]cache.Item)
	jsonFile, err := os.ReadFile(DumpFile)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal([]byte(jsonFile), &dumpObj)
	if err != nil {
		return 0, err
	}

	Pool = cache.NewFrom(Exp, Cleanup, dumpObj)
	return len(dumpObj), err
}

// itemsFromPost loads kv pairs from a json POST to the in memory database
func loadFromJSON(j []byte) (int, error) {
	dumpObj := make(map[string]cache.Item)
	err := json.Unmarshal(j, &dumpObj)
	if err != nil {
		return 0, err
	}

	Pool = cache.NewFrom(Exp, Cleanup, dumpObj)
	return len(dumpObj), nil
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

// IndexHandler return a fasthttp.RequestHandler function that genetares the index page
func IndexHandler(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		t.Execute(ctx, body{
			HasForm: true,
			Line1:   "Welcome to Short, the simple URL shortener,",
			Line2:   "Type an URL below to shorten it",
		})
	}
}

// Short return a fasthttp.RequestHandler function that genetares the shortener page
func Short(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		suf, err := shortener(string(ctx.FormValue("url")), URLSize)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			t.Execute(ctx, body{
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
		ru, _ := url.Parse(fmt.Sprintf("%s://%s:%v/%s%s", Proto, Domain, Port, Path, suf))
		t.Execute(ctx, body{
			IsLink: true,
			Line1:  ru.String(),
		})
	}
}

// Redir return a fasthttp.RequestHandler function that genetares the shortener redirect page
func Redir(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		key := ctx.UserValue("key")
		if Path != "" {
			key = strings.Replace(key.(string), Path, "", 1)
		}
		u, err := redirect(key.(string))
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			t.Execute(ctx, body{
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
		ctx.Redirect(u, fasthttp.StatusFound)
	}
}

// ToFile return a fasthttp.RequestHandler function that dumps the contents of the
// KV db to the DumpFile file
func ToFile(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		f, err := os.Create(DumpFile)
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
			t.Execute(ctx, internalError("Failed to create DB dump file", err))
			return
		}
		i, err := dumpDbTOFile(f)
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
			t.Execute(ctx, internalError("Failed to dump kv DB to file", err))
			return
		}
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		t.Execute(ctx, body{Line1: fmt.Sprintf("Exported %v items to %v", i, DumpFile)})
	}
}

// FromFile return a fasthttp.RequestHandler function that loads the content of the DumpFile into the
// KV db
func FromFile(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		i, err := loadFromFile()
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
			t.Execute(ctx, internalError("Error loading DB from file", err))
			return
		}
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		t.Execute(ctx, body{Line1: fmt.Sprintf("Imported %v items to the DB", i)})
	}
}

// Dump return a fasthttp.RequestHandler function that dumps the contents of the
// KV db to the ctx handler
func Dump(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		dumpObj, err := json.Marshal(Pool.Items())
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
			t.Execute(ctx, internalError("Unable to dump key value db: ", err))
			return
		}
		fmt.Fprintf(ctx, "%s", dumpObj)
	}
}

// FromPost return a fasthttp.RequestHandler function that loads the content of the JSON POST into the
// KV db
func FromPost(t *template.Template) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		i, err := loadFromJSON(ctx.PostBody())
		if err != nil {
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
			t.Execute(ctx, internalError("Error loading DB", err))
			return
		}
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("text/html"))
		t.Execute(ctx, body{Line1: fmt.Sprintf("Imported %v items to the DB", i)})
	}
}
