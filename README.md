# adspots


## Running

This project uses Go 1.25.1. With a recent enough Go toolchain installed, run
`make run` to run the server, and `make check` to run the tests.

## Tradeoffs

- `gorm` handles database interactions
  - As mentioned in the postmortem, an ORM is probably overkill for a project this small.

## What was been implemented

This is my solution for lane A of the take home challenge.

- The main server executable resides in `cmd/adspots/main.go`, and takes care of its lifecycle.
- The logic for each endpoint resides in `routes.go`.
- The types used in the project reside in `types.go`
- The rate-limiter implemented resides in `ratelimit.go`.
- Tests are in `t/`

- TTL business logic testing and end-to-end tests for filtering endpoint `(GET /adspots)`
  (stretch 2)
- Rate-limiting using a token bucket in memory (stretch 3)
- `Makefile` for project (`make run` and `make check`, stretch 4)
