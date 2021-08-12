FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/fiddle"]
CMD ["--port=:8000", "--prom_port=:20000", "--resources_dir=/usr/local/share/fiddle/", "--source_image_dir=/etc/fiddle/source"]

