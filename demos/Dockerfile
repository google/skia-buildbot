FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/demosfe"]
CMD ["--logtostderr", "--port=:8000", "--resources_dir=/usr/local/share/demosfe/"]

