FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk update && apk upgrade && \
    apk add --no-cache \
    bash \
    git \
    python

RUN mkdir -m 777 /opt/

USER skia

RUN git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git' /opt/depot_tools

COPY . /

ENTRYPOINT ["/usr/local/bin/fuzzer-fe"]
CMD ["--logtostderr", "--port=:8000", "--prom_port=:20000", "--resources_dir=/usr/local/share/fuzzer-fe/", \
	"--bolt_db_path=/mnt/pd0/fe-db", "--host=fuzzer.skia.org", "--skia_root=/mnt/pd0/skia-fe", \
	"--depot_tools_path=/mnt/pd0/depot_tools", "--fuzz_sync_period=1m0s", "--download_processes=32", \
	"--backend_names=skia-fuzzer-be-0", "--backend_names=skia-fuzzer-be-1"]

