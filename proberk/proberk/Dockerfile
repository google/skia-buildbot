FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/proberk"]
CMD ["--config=/etc/proberk/allprobersk.json", "--prom_port=:20000", "--run_every=10s"]
