ARG base
FROM $base
RUN apk add bash curl git openjdk8 procps python2 python3
RUN ln -s /usr/bin/python3 /usr/bin/python
ARG sdk_version
RUN echo $sdk_version
RUN cd /usr/local && \
    curl https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${sdk_version}-linux-x86_64.tar.gz | \
    tar -zxvf - google-cloud-sdk && \
    ls -alh /usr/local/google-cloud-sdk && \
    /usr/local/google-cloud-sdk/install.sh
ENV PATH /usr/local/google-cloud-sdk/bin:${PATH}
RUN gcloud components install \
    beta \
    cloud-firestore-emulator \
    bigtable \
    cloud-datastore-emulator \
    pubsub-emulator
RUN rm -f /usr/bin/python && ln -s /usr/bin/python2 /usr/bin/python
RUN mkdir /tmp/cockroach && \
    cd /tmp/cockroach && \
    curl https://binaries.cockroachdb.com/cockroach-v19.2.4.linux-musl-amd64.tgz | \
    tar -xzf - && \
    mv /tmp/cockroach/cockroach-v19.2.4.linux-musl-amd64/cockroach /bin/cockroach && \
    rm -rf /tmp/cockroach