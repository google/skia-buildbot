FROM gcr.io/skia-public/basealpine:3.8

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/k8s-checker"]
CMD ["--prom_port=:20000"]
