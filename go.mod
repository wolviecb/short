module github.com/wolviecb/short

go 1.19

require (
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/fasthttp/router v1.4.12
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/valyala/fasthttp v1.41.0
	internal/shortie v0.0.0-00010101000000-000000000000
)

replace internal/shortie => ./internal/shortie

require (
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/savsgio/gotils v0.0.0-20220530130905-52f3993e8d6d // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
)
