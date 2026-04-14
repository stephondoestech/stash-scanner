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

If you enable `identify` later, keep it opt-in. You only need `STASH_SCANNER_IDENTIFY_*` variables when Stash does not already have default identify sources configured.

3. Start the app:

```sh
make run-ui
```

4. Open the UI:

```text
http://127.0.0.1:8088/
```

Reviewer UI:

```sh
make run-reviewer
```

```text
http://127.0.0.1:8090/
```

The reviewer can now handle two manual workflows:

- assign performers to scenes or galleries that are missing them
- repair linked performers that already exist in Stash but are incomplete

Incomplete performers are currently defined as performers missing `name`, `gender`, and `image`.

## First Run Warning

Before the first real scanner run, make sure your Stash instance has recently scanned the same library paths and already knows about the files on disk.

The scanner builds its initial baseline from the filesystem, not from Stash. If files exist on disk but are missing from Stash during that first run, those files can be written into the scanner state without being revisited on later runs unless they change again.

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
- `STASH_SCANNER_DEBUG` - when `true`, include verbose debug logs for config, target selection, retries, and Stash polling
- `STASH_SCANNER_POST_SCAN_TASKS` - optional follow-up tasks after a successful scan; supported values are `auto_tag`, `identify`, and `clean`
- `STASH_SCANNER_STATE_PATH` - where the scanner stores its saved state and retry info; for Docker or Unraid, use `/config/state.json`
- `STASH_SCANNER_CONTROL_BIND` - the address and port for the built-in UI and API

Optional:

- `STASH_SCANNER_WATCH_ROOTS` - manual library paths if you do not want to pull them from Stash
- `STASH_SCANNER_IDENTIFY_STASH_BOX_INDEXES` - stash box indexes to use when `identify` is enabled
- `STASH_SCANNER_IDENTIFY_STASH_BOX_ENDPOINTS` - stash box endpoints to use when `identify` is enabled
- `STASH_SCANNER_IDENTIFY_SCRAPER_IDS` - scraper ids to use when `identify` is enabled
- `STASH_SCANNER_POST_SCAN_CLEAN_DRY_RUN` - when `true`, run the `clean` post-scan task as a dry run
- `STASH_SCANNER_CONTROL_FALLBACK_BIND` - backup address and port if the main UI bind fails
- `STASH_SCANNER_INTERVAL` - how often the scanner checks for changes
- `STASH_SCANNER_RETRY_MAX_ATTEMPTS` - maximum retry count for failed scan requests
- `STASH_SCANNER_RETRY_INITIAL_BACKOFF` - how long to wait before the first retry
- `STASH_SCANNER_RETRY_MAX_BACKOFF` - the longest retry delay the scanner will use

Docker and `.env` guidance:

- `STASH_SCANNER_STASH_URL` and `STASH_SCANNER_API_KEY` are required for normal scanner and reviewer operation
- `STASH_REVIEWER_MIN_SCORE` and `STASH_REVIEWER_MIN_LEAD` are optional; defaults are used when unset
- `STASH_SCANNER_IDENTIFY_*` values are optional unless `identify` is enabled and Stash has no default identify sources configured
- reviewer-specific `STASH_REVIEWER_STASH_URL` and `STASH_REVIEWER_API_KEY` are optional overrides; they fall back to the scanner values

Reviewer runtime:

- `STASH_REVIEWER_STASH_URL` - optional override for the reviewer Stash URL; falls back to `STASH_SCANNER_STASH_URL`
- `STASH_REVIEWER_API_KEY` - optional override for the reviewer API key; falls back to `STASH_SCANNER_API_KEY`
- `STASH_REVIEWER_BIND` - bind address for the reviewer UI, default `127.0.0.1:8090`
- `STASH_REVIEWER_QUEUE_PATH` - where the reviewer stores its local queue snapshot, default `data/reviewer-queue.json`
- `STASH_REVIEWER_REFRESH_INTERVAL` - optional background refresh interval; by default the reviewer refreshes once at startup and on manual request only
- `STASH_REVIEWER_MIN_SCORE` - minimum reviewer candidate score required before a suggestion is shown
- `STASH_REVIEWER_MIN_LEAD` - minimum score lead the top candidate must have over the runner-up to avoid suppression as ambiguous
- the reviewer UI can also adjust the active thresholds at runtime; the change applies immediately by refreshing the queue with the new settings
- reviewer repair attempts are manual only; the UI exposes a repair button for incomplete linked performers or incomplete manual-search results when a stash-id-backed repair is possible

## Commands

```sh
make run-ui
make run
make run-once
make run-reviewer
go run ./cmd/scanner -requeue-paths /path/one,/path/two
make test
make docker-build
```

Use `-requeue-paths` when files were written into the scanner state but never made it into Stash. The command removes tracked entries beneath those paths from the state file and exits; run the scanner again afterward so it can rediscover and rescan them.

## UI And API

UI:

- `GET /`

API:

- `GET /api/status`
- `POST /api/run-now`
- `POST /api/flush-debounce`

## Notes

- `dry_run=true` means the scanner logs what it would do but does not send a real scan request to Stash.
- `dry_run=false` sends real `metadataScan` requests to Stash.
- `debug=true` enables verbose operational logging for troubleshooting.
- a successful debounce target run always performs the selective `metadataScan` first; post-scan tasks then run in this order: `identify`, `auto_tag`, `clean`
- `identify` now auto-discovers only Stash's configured default identify sources when local `STASH_SCANNER_IDENTIFY_*` overrides are not set.
- if Stash has no default identify sources configured, `identify` now fails fast and requires explicit `STASH_SCANNER_IDENTIFY_*` settings instead of widening to every discovered stash box or scraper.
- keep `identify` opt-in in deployment configs unless you have reviewed the active default sources or set explicit `STASH_SCANNER_IDENTIFY_*` values.
- `post_scan_tasks=auto_tag,identify` still works, but task execution is normalized to `identify` before `auto_tag`
- if the control port cannot bind, the app either uses `control.fallback_bind` or exits with a clear error.
- the Docker image defaults `STASH_SCANNER_STATE_PATH` to `/config/state.json`, so mount a writable host path to `/config` for persistence.
- scheduled mode waits for the first configured interval or daily time; it does not auto-run a scan immediately on service startup
- the control UI can promote pending debounce paths for immediate processing with `Scan Pending Now`
- the scanner control UI now shows the resolved identify sources used for `identify` post-scan tasks

## Project Notes

- commit messages should follow the Commitizen / Conventional Commits style used by this repo
- this project was built with assistance from OpenAI Codex for transparency

## Known Issues

- Current state file grows with the size of the Stash instance so it can easily exceed 1 GB or more. Will work on solving this in a future release. 
- If the first run happened before Stash was fully up to date, files may exist in the scanner state but still be missing from Stash. To recover, delete the relevant entries from the state file so the scanner can rediscover them, or run a Stash scan that covers those paths directly.
- The reviewer candidate engine remains heuristic for matching, but reviewer actions can now write back to Stash for manual assignments and manual performer repair attempts.
