FROM gcr.io/skia-public/basedebian:testing-slim

USER skia

COPY . /

ENV PATH="/mnt/pd0/status_workdir/git/bin:${PATH}"

ENTRYPOINT ["/usr/local/bin/statusk"]
