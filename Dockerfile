FROM golang:1.9-alpine AS builder
WORKDIR /go/src/github.com/bdclark/nomadctl
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nomadctl .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /go/src/github.com/bdclark/nomadctl/nomadctl .
ENTRYPOINT ["./nomadctl"]
