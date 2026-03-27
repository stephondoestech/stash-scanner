FROM golang:1.25.7-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY VERSION ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/scanner ./cmd/scanner

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/scanner /usr/local/bin/scanner
COPY VERSION /app/VERSION

RUN mkdir -p /app/data

ENTRYPOINT ["/usr/local/bin/scanner"]
CMD []
