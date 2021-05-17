FROM golang:1.16.3 as builder

WORKDIR /go/src

COPY go.mod go.sum ./
RUN go mod download

COPY ./main.go  ./

ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64
RUN go build \
    -o /go/bin/main \
    -ldflags '-s -w'

FROM alpine:3.13.5 as runner

COPY --from=builder /go/bin/main /app/main
EXPOSE 5000

ENTRYPOINT ["/app/main"]
