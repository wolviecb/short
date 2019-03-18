package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"regexp"
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
	appVersion = "0.0.1"
)

var domain string
var redisServer string
var listenAddr string
var src = rand.NewSource(time.Now().UnixNano())
var pool = newPool()

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
	// return fmt.Sprintf("%v\n", ctx.Params)
	u, err := url.ParseRequestURI(ctx.Params["url"])
	if err != nil {
		ctx.Abort(400, "Bad URL")
	} else {
		suffix := RandStringBytesMaskImprSrc(10)
		// nURL := "https://" + domain + "/" + suffix
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
		err := set(conn, u.String(), suffix)
		if err != nil {
			ctx.Abort(500, "Internal Error")
		} else {
			ctx.WriteString("URL shortened at: https://" + domain + "/" + suffix)
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
	version := flag.Bool("v", false, "prints current roxy version")
	flag.Parse()
	if *version {
		fmt.Printf("%s", appVersion)
		os.Exit(0)
	}

	web.Post("/", shortner)
	web.Get("/(.*)", redirect)
	log.Printf("Domain: %s, Redis: %s\n", domain, redisServer)
	web.Run(listenAddr)
}
