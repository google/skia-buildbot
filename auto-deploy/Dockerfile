FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

# Install kubctl (needed for pushk).
RUN apt-get update && apt-get upgrade -y && apt-get install -y curl bash software-properties-common gnupg2 \
    && mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/auto-deploy"]
CMD ["--logtostderr"]
