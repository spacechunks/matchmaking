FROM golang:1.26.3-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
COPY vendor .
COPY .. .
RUN mkdir bin
RUN go build -mod vendor -o bin ./cmd/mm

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
WORKDIR /bin
COPY --from=builder /build/bin/mm mm
ENTRYPOINT ["/bin/mm"]
