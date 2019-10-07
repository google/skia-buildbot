FROM gcr.io/skia-public/basedebian:testing-slim

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/k8s_checker"]
CMD ["--logtostderr", "--prom_port=:20000"]
