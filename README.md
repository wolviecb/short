Short - simple go url shortner
==============================

[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://git.thebarrens.nu/wolvie/short/blob/master/LICENSE)
[![Build Status](https://git.thebarrens.nu/wolvie/short/badges/master/build.svg)](https://git.thebarrens.nu/wolvie/short/)
Short is a very simple url shortener build in golang using web.go module for storring URLs, the main focus is speed, not data is persisted.


Syntax is:

```shell
Usage of short:
  -addr string
        Address to listen for connections (default "localhost:8080")
  -domain string
        Domain to write to the URLs (default "localhost")
  -path string
        Path to the base URL (https://localhost/PATH/... remember to append a / at the end
  -proto string
        proto to the base URL (HTTPS://localhost/path/... no real https here just to set the url (for like a proxy offloading https (default "https")
  -redis string
        ip/hostname of the redis server to connect (default "localhost:6379")
  -v    prints current version

```

Includes a Dockerfile to for a standalone docker image.

To shorten a URL just post on /, you will get a reply with the shortened URL

```shell
curl -X POST -d "url=http://google.com" http://localhost:8080/
URL shortened at: https://localhost:8080/9mbIcOwsVP
```
