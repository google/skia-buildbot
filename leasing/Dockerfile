FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && apk add --no-cache git

USER skia

COPY . /

ENTRYPOINT ["/leasing"]
# TODO(rmistry): Will have to add more here.
CMD ["--logtostderr", "--prom_port=:20000", "--resources_dir=/usr/local/share/leasing/"]

