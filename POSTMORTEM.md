# Postmortem

## Forseeable risks

The TTL expiry calculation is based on calendar time, which is prone to failure
when the wall clock jumps around. Cloudflare famously had an outage in 2017 due
to this: [How and why the leap second affected Cloudflare
DNS](https://blog.cloudflare.com/how-and-why-the-leap-second-affected-cloudflare-dns/)

The rate limiting code uses mutexes for safe concurrency, this could lead to
contention.

Perhaps using an ORM for a project this small could be overkill, but it does aid
in making the code less prone to errors that could occur when writing procedures
to scan rows from `database/sql` drivers.

## Productionizing

Adding proper metrics and/or logging is a must before productionizing this kind
of service. Extending the integration tests to all endpoints would also be nice.
