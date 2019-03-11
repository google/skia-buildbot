FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/particles"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/particles"]
