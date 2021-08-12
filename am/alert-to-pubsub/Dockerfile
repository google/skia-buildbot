FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/alert-to-pubsub"]
CMD ["--prom_port=:20000", "--port=:8000"]
