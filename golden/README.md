Gold Correctness
================

For information on setting up or using Gold, see [these docs](docs/README.md).

For an architectural overview, see:
<https://docs.google.com/document/d/1U7eBzYrZCPx24Lp9JH2scKj3G8Gr8GRtQJZRhdigyRQ/edit>

To run Gold locally, see:
<https://skia.googlesource.com/infra-internal/+show/c6fad0bec78c6768ce7e4187606325216dd438ed/scripts/start-gold-chrome-gpu.sh>

Backend Storage
---------------

Gold uses [CockroachDB](https://www.cockroachlabs.com/get-cockroachdb/) to store all data necessary
for running the backend servers. (Caveat: We are in the middle of a migration towards this goal.)

For production-specific advice, see docs/PROD.md.
