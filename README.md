# Short - simple go url shortener

[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://git.thebarrens.nu/wolvie/short/blob/master/LICENSE)

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
      Path to the base URL (https://localhost/PATH/... remember to append a / at the end
  -port string
      Port to listen for connections (default "8080")
  -proto string
      proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https (default "https")
  -urlsize int
      Define the size of the shortened String, default 10 (default 10)
  -v  prints current version
```

Includes a Dockerfile to for a standalone docker image.

To shorten a URL just post on /, you will get a reply with the shortened URL

```shell
curl -X POST -d "url=http://google.com" http://localhost:8080/
[...]
URL shortened at: https://localhost:8080/9mbIcOwsVP
[...]
```

URLs missing a Scheme (http[s]://) will be defaulted to https

## Dump/Restore endpoints

URLs for mapping data can be checked, listed, dumped and restored in the given endpoints (you might want (and you should) restrict access to this e.g. reverse proxy):

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

```shell
curl http://localhost:8080/v1/dumpToFile
```

Load url mappings from `-dump` file to in memory db

```shell
curl http://localhost:8080/v1/fromFile
```

Load url mappings from POST data (Assuming json data on save.json file)

```shell
curl -X POST http://localhost:8080/v1/fromPost \
-H "Content-Type: application/json" \
--data $(cat save.json )
```

## HTML templates

A simple collection of html templates are put on `templates` folder, the templates I've used are based on [@jspark721]( https://github.com/jspark721 ) "UI 404 PAGE" on [freefrontend](https://codepen.io/juliepark/pen/erOoeZ)
