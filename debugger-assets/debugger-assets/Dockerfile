FROM gcr.io/skia-public/basealpine:3.7

ADD . /

USER skia

ENTRYPOINT ["/bin/debugger-assets"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger-assets"]
