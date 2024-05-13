FROM golang:1.22 as builder

ENV CGO_ENABLED=0

COPY . /go/src/short/

WORKDIR /go/src/short

RUN go get ./... && go build -o short .

FROM scratch

LABEL  maintainer="Thomas Andrade <wolvie@gmail.com>"

COPY --from=builder /go/src/short/short /
COPY templates /templates

ENTRYPOINT [ "/short" ]
