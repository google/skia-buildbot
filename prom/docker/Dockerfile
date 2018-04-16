FROM alpine

ADD . /

CMD ["/configmap-reload", "--volume-dir=/etc/prometheus/", "--webhook-method=POST", "--webhook-url=http://localhost:9090/-/reload"]
