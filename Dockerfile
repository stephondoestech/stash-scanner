FROM golang:1.25.7-alpine AS build

WORKDIR /src

ARG GIT_COMMIT=unknown

COPY go.mod ./
COPY VERSION ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-X 'stash-scanner/internal/version.commit=${GIT_COMMIT}'" \
  -o /out/scanner ./cmd/scanner

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/scanner /usr/local/bin/scanner
COPY VERSION /app/VERSION

RUN mkdir -p /config

ENV STASH_SCANNER_STATE_PATH=/config/state.json

ENTRYPOINT ["/usr/local/bin/scanner"]
CMD []
