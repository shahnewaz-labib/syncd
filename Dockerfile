FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY . .

RUN go build -o syncd /app/cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/syncd .

ENTRYPOINT ["./syncd"]
