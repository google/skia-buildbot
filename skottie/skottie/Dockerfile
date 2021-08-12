FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/skottie"]
CMD ["--resources_dir=/usr/local/share/skottie"]
