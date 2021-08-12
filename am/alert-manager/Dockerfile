FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/alert-manager"]
CMD ["--prom_port=:20001", "--namespace=am-localhost-jcgregorio", "--port=:8080", "--resources_dir=/usr/local/share/alert-manager/"]
