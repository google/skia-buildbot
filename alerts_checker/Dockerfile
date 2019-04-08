FROM gcr.io/skia-public/basedebian:testing-slim

USER root

# Install kubectl.
RUN apt-get update && apt-get install -y apt-transport-https curl gnupg
RUN curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
RUN echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list
RUN apt-get update
RUN apt-get install -y kubectl

# Install other required packages.
RUN apt-get install -y jq

# Install other useful packages.
RUN apt-get install -y procps curl vim jq

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/alerts_checker"]
CMD ["--logtostderr", "--prom_port=:20000"]
