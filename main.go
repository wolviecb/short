package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/hoisie/web"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Base strings for RandStringBytesMaskImprSrc
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)
const (
	appVersion = "0.0.3"
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
var listenAddr string
var proto string
var path string
var src = rand.NewSource(time.Now().UnixNano())
var pool = newPool()

func index() string { return indexPage }
func newPool() *redis.Pool {
	return &redis.Pool{
		// Maximum number of idle connections in the pool.
		MaxIdle: 10,
		// max number of connections
		MaxActive: 1200,
		// Dial is an application supplied function for creating and
		// configuring a connection.
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisServer)
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

// ping tests connectivity for redis (PONG should be returned)
func ping(c redis.Conn) error {
	// Send PING command to Redis
	// PING command returns a Redis "Simple String"
	// Use redis.String to convert the interface type to string
	_, err := redis.String(c.Do("PING"))
	if err != nil {
		return err
	}
	// fmt.Printf("PING Response = %s\n", s)
	// Output: PONG
	return nil
}

// get executes the redis GET command
func get(c redis.Conn, key string) (bool, string) {
	s, err := redis.String(c.Do("GET", key))
	if err == redis.ErrNil {
		return false, ""
	} else if err != nil {
		return false, ""
	} else {
		return true, s
	}
}

// set executes the redis SET command
func set(c redis.Conn, url, suffix string) error {
	_, err := c.Do("SET", suffix, url)
	if err != nil {
		return err
	}
	return nil
}

func redirect(ctx *web.Context, val string) {
	if path != "" {
		val = strings.Replace(val, path, "", 1)
	}
	r, _ := regexp.Compile("[a-zA-Z0-9]+")
	key := r.FindString(val)
	conn := pool.Get()
	defer conn.Close()
	status, url := get(conn, key)
	if status {
		ctx.Redirect(302, url)
	} else {
		ctx.NotFound("URL don't exist")
	}
}

func shortner(ctx *web.Context) {
	_, port, _ := net.SplitHostPort(listenAddr)
	u, err := url.ParseRequestURI(ctx.Params["url"])
	if err != nil {
		ctx.Abort(400, "Bad URL")
	} else {
		suffix := RandStringBytesMaskImprSrc(10)
		conn := pool.Get()
		defer conn.Close()
		for {
			status, _ := get(conn, suffix)
			if status {
				suffix = RandStringBytesMaskImprSrc(10)
			} else {
				break
			}
		}
		if port != "80" && proto == "http" {
			port = ":" + port + "/"
		} else if port != "443" && proto == "https" {
			port = ":" + port + "/"
		} else if port == "443" || port == "80" {
			port = "/"
		}
		err := set(conn, u.String(), suffix)
		if err != nil {
			ctx.Abort(500, "Internal Error")
		} else {
			shortend := proto + "://" + domain + port + path + suffix
			output := fmt.Sprintf(returnPage, shortend, shortend)
			ctx.WriteString(output)
		}
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

func main() {
	flag.StringVar(&domain, "domain", "localhost", "Domain to write to the URLs")
	flag.StringVar(&redisServer, "redis", "localhost:6379", "ip/hostname of the redis server to connect")
	flag.StringVar(&listenAddr, "addr", "localhost:8080", "Address to listen for connections")
	flag.StringVar(&path, "path", "", "Path to the base URL (https://localhost/PATH/... remember to append a / at the end")
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

	web.Get("/", index)
	web.Post("/", shortner)
	web.Get("/(.*)", redirect)
	log.Printf("Domain: %s, Redis: %s\n", domain, redisServer)
	web.Run(listenAddr)
}
