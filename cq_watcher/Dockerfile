FROM gcr.io/skia-public/basealpine:3.8

USER skia

COPY . /

ENTRYPOINT ["/cq_watcher"]
CMD ["--prom_port=:20000"]
