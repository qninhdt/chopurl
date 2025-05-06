# syntax=docker/dockerfile:1

FROM golang:1.24

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build

EXPOSE 8080

# Run
CMD ["./url-shorten-service"]