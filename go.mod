module github.com/wolviecb/short

go 1.21

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/fasthttp/router v1.5.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/valyala/fasthttp v1.57.0
	internal/shortie v0.0.0-00010101000000-000000000000
)

replace internal/shortie => ./internal/shortie

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/savsgio/gotils v0.0.0-20240303185622-093b76447511 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
)
