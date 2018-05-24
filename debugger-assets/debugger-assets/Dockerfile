FROM gcr.io/skia-public/basealpine:3.7

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/debugger-assets"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger-assets"]
