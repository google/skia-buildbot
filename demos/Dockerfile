FROM gcr.io/skia-public/base-cipd:testing-slim

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/demoserver"]
CMD ["--logtostderr", "--port=:8000", \
        "--resources_dir=/usr/local/share/demoserver/dist", \
        "--demos_dir=/usr/local/share/demoserver/demos"]

