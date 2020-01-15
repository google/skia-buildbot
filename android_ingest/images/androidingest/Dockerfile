FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y file ca-certificates

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/androidingest"]
CMD ["--logtostderr"]
