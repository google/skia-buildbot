FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk update && \
    apk add --no-cache bash procps

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/tree_status"]
CMD ["--prom_port=:20001", "--namespace=tree-status-staging", "--port=:8080", "--resources_dir=/usr/local/share/tree_status/"]
