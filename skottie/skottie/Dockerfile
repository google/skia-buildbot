FROM gcr.io/skia-public/basealpine:3.7

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/skottie"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/skottie"]
