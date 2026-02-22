FROM golang:1.26-alpine

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY . .

RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1

RUN go mod download

RUN golangci-lint custom

ENTRYPOINT ["./bin/custom-gcl", "run", "testdata/logs.go"]