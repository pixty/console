FROM golang:1.7

RUN mkdir -p /go/src/github.com/pixty/console
WORKDIR /go/src/github.com/pixty/console

# this will ideally be built by the ONBUILD below ;)
CMD ["console", "-debug"]

COPY . /go/src/github.com/pixty/console
RUN go-wrapper download
RUN go-wrapper install
