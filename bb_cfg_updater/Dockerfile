FROM gcr.io/skia-public/basealpine:3.8

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/bb-cfg-updater"]
CMD ["--logtostderr", "--prom_port=:20000"]
