FROM gcr.io/skia-public/base-cipd:testing-slim

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/debugger-app"]
CMD ["--logtostderr", "--port=:8000", \
     "--resources_dir=/usr/local/share/debugger/"]

