Short - simple go url shortner
==============================

[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://git.thebarrens.nu/wolvie/short/blob/master/LICENSE)
[![Build Status](https://git.thebarrens.nu/wolvie/short/badges/master/build.svg)](https://git.thebarrens.nu/wolvie/short/)

Short is a very simple url shortener build in golang using web.go module and redis (redigo) for storring URLs

Syntax is:

```shell
Usage of /short:
  -addr string
        Address to listen for connections (default "localhost:8080")
  -domain string
        Domain to write to the URLs (default "localhost")
  -redis string
        ip/hostname of the redis server to connect (default "localhost:6379")
  -v    prints current roxy version
```

Includes a Dockerfile to for a standalone docker image