FROM gcr.io/skia-public/basealpine:3.8

USER root

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/api"]
CMD ["--resources_dir=/usr/local/share/api/docs/"]
