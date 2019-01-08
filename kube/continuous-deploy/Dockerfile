FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk --no-cache add curl git \
    && mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/continuous-deploy"]
CMD ["--logtostderr", "fiddler"]
