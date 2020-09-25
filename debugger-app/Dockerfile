#TODO(nifong): make this work, pull stuff from debugger-assets

FROM gcr.io/skia-public/base-cipd:testing-slim

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/debugger"]
CMD ["--logtostderr", "--port=:8000", \
        "--resources_dir=/usr/local/share/debugger/dist", \
        "--repo_url=https://skia.googlesource.com/infra-internal"]

