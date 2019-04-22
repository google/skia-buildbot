FROM gcr.io/skia-public/basedebian:testing-slim

USER root

# Install kubectl and other useful packages.
RUN apt-get update && apt-get install -y apt-transport-https curl gnupg procps vim \
  && curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list \
  && apt-get update \
  && apt-get install -y kubectl \
  && rm -rf /var/lib/apt/lists/*

# Install the gcloud package.
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz \
  && tar --directory /usr/lib/ -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/lib/google-cloud-sdk/install.sh \
  && rm /tmp/google-cloud-sdk.tar.gz
ENV PATH $PATH:/usr/lib/google-cloud-sdk/bin

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/k8s_checker"]
CMD ["--logtostderr", "--prom_port=:20000"]
