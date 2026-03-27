# Stash Scanner

`stash-scanner` is a sidecar app for Stash.

It watches your library for changes and asks Stash to run targeted scans, instead of doing a full library scan every time.

## What It Does

- watches your Stash library paths
- detects new or changed files
- sends path-based scan requests to Stash
- gives you a small control UI with status and `Run Now`

## Recommended Setup

Use Stash as the source of truth for library paths.

Set this in [`.env.example`](/Users/stephonparker/Dev/stash-scanner/.env.example):

```dotenv
STASH_SCANNER_WATCH_ROOTS_FROM_STASH=true
```

That tells the scanner to read library paths from:

`configuration.general.stashes[].path`

on your Stash instance.

## Important Rule

The scanner must see the same library paths that Stash sees.

Example:

- if Stash uses `/mnt/user/data/media`
- the scanner container must also mount that library as `/mnt/user/data/media`

Do not remap it to a different container path like `/media` if you want automatic root discovery to work.

## Quick Start

1. Copy the env template:

```sh
cp .env.example .env
```

2. Edit `.env` and set at least:

```dotenv
STASH_SCANNER_STASH_URL=http://localhost:9999
STASH_SCANNER_API_KEY=replace-me
STASH_SCANNER_WATCH_ROOTS_FROM_STASH=true
STASH_SCANNER_DRY_RUN=false
```

3. Start the app:

```sh
make run-ui
```

4. Open the UI:

```text
http://127.0.0.1:8088/
```

## Unraid

For Unraid, the recommended pattern is:

- store scanner state and app data in `appdata`
- mount your media with the same container path Stash uses
- keep settings in `.env`

Typical mappings:

- `/mnt/user/appdata/stash-scanner` -> `/config`
- `/mnt/user/data` -> `/mnt/user/data`

Recommended Unraid setting:

```dotenv
STASH_SCANNER_STATE_PATH=/config/state.json
```

Example Compose file: [`docker-compose.example.yml`](/Users/stephonparker/Dev/stash-scanner/docker-compose.example.yml)

## Main Settings

These are the ones most users care about:

- `STASH_SCANNER_STASH_URL` - the URL for your Stash instance
- `STASH_SCANNER_API_KEY` - the API key the scanner uses to talk to Stash
- `STASH_SCANNER_WATCH_ROOTS_FROM_STASH` - tells the scanner to pull library paths directly from Stash
- `STASH_SCANNER_DRY_RUN` - when `true`, log actions without sending real scan requests
- `STASH_SCANNER_STATE_PATH` - where the scanner stores its saved state and retry info; for Docker or Unraid, use `/config/state.json`
- `STASH_SCANNER_CONTROL_BIND` - the address and port for the built-in UI and API

Optional:

- `STASH_SCANNER_WATCH_ROOTS` - manual library paths if you do not want to pull them from Stash
- `STASH_SCANNER_CONTROL_FALLBACK_BIND` - backup address and port if the main UI bind fails
- `STASH_SCANNER_INTERVAL` - how often the scanner checks for changes
- `STASH_SCANNER_RETRY_MAX_ATTEMPTS` - maximum retry count for failed scan requests
- `STASH_SCANNER_RETRY_INITIAL_BACKOFF` - how long to wait before the first retry
- `STASH_SCANNER_RETRY_MAX_BACKOFF` - the longest retry delay the scanner will use

## Commands

```sh
make run-ui
make run
make run-once
make test
make docker-build
```

## UI And API

UI:

- `GET /`

API:

- `GET /api/status`
- `POST /api/run-now`

## Notes

- `dry_run=true` means the scanner logs what it would do but does not send a real scan request to Stash.
- `dry_run=false` sends real `metadataScan` requests to Stash.
- if the control port cannot bind, the app either uses `control.fallback_bind` or exits with a clear error.
- the Docker image defaults `STASH_SCANNER_STATE_PATH` to `/config/state.json`, so mount a writable host path to `/config` for persistence.

## Project Notes

- commit messages should follow the Commitizen / Conventional Commits style used by this repo
- this project was built with assistance from OpenAI Codex for transparency
