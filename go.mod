module github.com/wolviecb/short

go 1.23.0

toolchain go1.25.0

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/fasthttp/router v1.5.4
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/valyala/fasthttp v1.65.0
	internal/shortie v0.0.0-00010101000000-000000000000
)

replace internal/shortie => ./internal/shortie

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/savsgio/gotils v0.0.0-20240704082632-aef3928b8a38 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
)
