FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/hashtag"]
CMD ["--logtostderr", "--prom_port=:20001", "--port=:8000", "--resources_dir=/usr/local/share/hashtag/"]