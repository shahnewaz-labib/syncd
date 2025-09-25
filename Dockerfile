FROM golang:1.25.1-alpine

WORKDIR /app
COPY . .
RUN go build -o syncd .

CMD ["./syncd"]
