# Short - simple go url shortner

[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://git.thebarrens.nu/wolvie/short/blob/master/LICENSE)
[![Build Status](https://git.thebarrens.nu/wolvie/short/badges/master/build.svg)](https://git.thebarrens.nu/wolvie/short/)
Short is a very simple url shortener build in golang using gorilla/mux for url routing and go-cache for storring URLs, the main focus is speed, not data is persisted, but can be dumped and restored.

## Syntax is

```shell
Usage of short:
  -addr string
    Address to listen for connections (default "localhost")
  -domain string
    Domain to write to the URLs (default "localhost")
  -dump string
    Path to the file to dump the kv db (default "urls.json")
  -path string
    Path to the base URL (https://localhost:8080/PATH/... remember to append a / at the end
  -port string
    Port to listen for connections (default "8080")
  -proto string
    proto to the base URL (HTTPS://localhost:8080/path/... no real https here just to set the url (for like a proxy offloading https (default "https")
  -v
    prints current version

```

Includes a Dockerfile to for a standalone docker image.

To shorten a URL just post on /, you will get a reply with the shortened URL

```shell
curl -X POST -d "url=http://google.com" http://localhost:8080/
URL shortened at: https://localhost:8080/9mbIcOwsVP
```

## Dump/Restore endpoints

URL mapping data can be checked, listed, dumped and restored in the given endpoints:

Show the number of mapped urls

```shell
$ curl http://localhost:8080/v1/count
X
```

Dumps the mapped url to json

```shell
$ curl http://localhost:8080/v1/dump
[...] #json of mapped urls
```

Dump the mapped url to `-dump` file (defaults to ./urls.json)

```shel
$ curl http://localhost:8080/v1/dumpToFile
Dump writen to: urls.json
```

Load url mappings from `-dimp` file to in memory db

```shell
$ curl http://localhost:8080/v1/fromFile
OK
```

Load url mappings from POST data (Assuming json data on save.json file)

```shell
$ curl -X POST http://localhost:8080/v1/fromPost \                                            ✔  0.59 L
-H "Content-Type: application/json" \
--data $(cat save.json )
OK
```