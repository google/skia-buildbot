FROM gcr.io/skia-public/base-cipd:testing-slim

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/demoserver"]
CMD ["--port=:8000", \
        "--resources_dir=/usr/local/share/demoserver/dist", \
        "--repo_url=https://skia.googlesource.com/infra-internal" \
        "--demos_dir=demos/internal"]

