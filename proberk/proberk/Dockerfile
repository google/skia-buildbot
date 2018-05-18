FROM gcr.io/skia-public/basealpine:3.7

ADD . /

USER skia

ENTRYPOINT ["/bin/proberk"]
CMD ["--logtostderr", "--config=/etc/proberk/allprobersk.json", "--prom_port=:20000", "--run_every=10s"]
