FROM gcr.io/skia-public/basealpine:3.7

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/alert-manager"]
CMD ["--logtostderr", "--port=:8000", "--prom_port=:20000", "--resources_dir=/usr/local/share/alert-manager/"]
