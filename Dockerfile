FROM golang:1.12.1 as builder

ENV CGO_ENABLED=0

ADD . /go/src/short

WORKDIR /go/src/short

RUN go get ./... && go build -o short .

FROM scratch

LABEL  maintainer="Thomas Andrade <wolvie@gmail.com>"

COPY --from=builder /go/src/short/short /

ENTRYPOINT [ "/short" ]