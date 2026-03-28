FROM golang:alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o syncd .

FROM alpine:3.19

RUN apk add --no-cache bash curl

WORKDIR /app
COPY --from=builder /app/syncd /usr/local/bin/syncd

# Create directories for testing
RUN mkdir -p /testfiles /downloads

ENTRYPOINT ["syncd"]
CMD ["daemon"]
