FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/named-fiddles"]
CMD ["--logtostderr", "--prom_port=:20000", "--resources_dir=/usr/local/share/named-fiddles/"]
