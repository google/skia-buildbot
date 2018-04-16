FROM alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

ADD . /

EXPOSE 8000 20000

CMD ["/prober", "--logtostderr", "--config=/etc/config/allprobers.json5"]
