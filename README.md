# adspots


## Running

This project uses Go 1.25.1.
With a recent enough Go toolchain installed, run `make run`.

## Tradeoffs

- `gorm` handles database interactions
  - As mentioned in the postmortem, an ORM is probably overkill for a project this small.

## What was been implemented

This is my solution for lane A of the take home challenge.

- TTL business logic testing and end-to-end tests for filtering endpoint `(GET /adspots)`
  (stretch 2)
- Rate-limiting using a token bucket in memory (stretch 3)
- `Makefile` for project (`make run` and `make check`, stretch 4)
