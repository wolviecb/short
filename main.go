package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"internal/shortie"

	"github.com/valyala/fasthttp"

	"github.com/asaskevich/govalidator"
	"github.com/fasthttp/router"
	"github.com/patrickmn/go-cache"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Base strings for randStringBytesMaskImprSrc
	letterIdxBits = 6                                                                // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1                                             // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits                                               // # of letter indices fitting in 63 bits
	appVersion    = "1.3.0"
)

var (
	out = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
)

func logger(r fasthttp.RequestHandler) fasthttp.RequestHandler {
	return fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		b := time.Now()
		r(ctx)
		e := time.Now()
		out.Printf("[%v] %v | %s | %s %s - %v - %v | %s",
			e.Format("2006/01/02 - 15:04:05"),
			ctx.RemoteAddr(),
			getHTTP(ctx),
			ctx.Method(),
			ctx.RequestURI(),
			ctx.Response.Header.StatusCode(),
			e.Sub(b),
			ctx.UserAgent(),
		)
	})
}

func getHTTP(ctx *fasthttp.RequestCtx) string {
	if ctx.Response.Header.IsHTTP11() {
		return "HTTP/1.1"
	}
	return "HTTP/1.0"
}

func healthz() func(ctx *fasthttp.RequestCtx) {
	r := struct {
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
	}{
		Status:     "ok",
		StatusCode: 200,
	}

	t := time.Now()
	shortie.Pool.Set("status", t.Unix(), -1)
	s, f := shortie.Pool.Get("status")

	if !f || s != t.Unix() {
		r.Status = "error"
		r.StatusCode = 500
	}

	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("application/json"))
		ctx.Response.SetStatusCode(r.StatusCode)
		if err := json.NewEncoder(ctx).Encode(r); err != nil {
			ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		}
	}
}

func main() {
	var (
		addr       = flag.String("addr", "localhost", "Address to listen for connections")
		domain     = flag.String("domain", "localhost", "Domain to write to the URLs")
		path       = flag.String("path", "", "Path to the base URL (https://localhost/PATH/...")
		http       = flag.Bool("http", false, "proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https")
		port       = flag.Int("port", 8080, "Port to listen for connections")
		exp        = flag.Int("exp", 240, "Default expiration time in hours, default 240")
		cleanup    = flag.Int("cleanup", 1, "Cleanup interval in hours, default 1")
		version    = flag.Bool("v", false, "prints current version")
		listenAddr string
	)
	flag.StringVar(&shortie.DumpFile, "dumpFile", "Path to the file to dump the kv db", "urls.json")
	flag.IntVar(&shortie.URLSize, "size", 10, "Define the size of the shortened String")
	flag.IntVar(&shortie.Port, "urlPort", 443, "Port to use for building URLs")

	flag.Parse()

	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	if shortie.Port > 65535 || shortie.Port < 1 {
		log.Fatalln("Invalid port number")
	}

	if *port > 65535 || *port < 1 {
		log.Fatalln("Invalid port number")
	}

	if *path != "" && !strings.HasSuffix(*path, "/") {
		*path = *path + "/"
	}
	shortie.Path = *path

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

	if *http {
		shortie.Proto = "http"
	}
	shortie.Domain = *domain
	shortie.Exp = time.Duration(*exp) * time.Hour
	shortie.Cleanup = time.Duration(*cleanup) * time.Hour

	shortie.Pool = cache.New(shortie.Exp, shortie.Cleanup)
	t := template.Must(template.ParseFiles("templates/response.html"))
	r := router.New()

	r.GET("/", shortie.IndexHandler(t))
	r.POST("/", shortie.Short(t))
	r.GET("/{key}", shortie.Redir(t))
	r.GET("/healthz", healthz())
	r.GET("/v1/toFile", shortie.ToFile(t))
	r.GET("/v1/fromFile", shortie.FromFile(t))
	r.GET("/v1/count", func(ctx *fasthttp.RequestCtx) { fmt.Fprintf(ctx, "%v", shortie.Pool.ItemCount()) })
	r.GET("/v1/dump", shortie.Dump(t))
	r.POST("/v1/fromPost", shortie.FromPost(t))

	log.Printf("Domain: %s, URL Proto: %s, Listen Address: %s\n", shortie.Domain, shortie.Proto, listenAddr)
	log.Fatal(fasthttp.ListenAndServe(listenAddr, logger(r.Handler)))
}
