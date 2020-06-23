ARG base
FROM $base
RUN apk add procps openjdk8
RUN gcloud components install \
    beta \
    cloud-firestore-emulator \
    bigtable \
    cloud-datastore-emulator \
    pubsub-emulator
RUN mkdir /tmp/cockroach && \
    cd /tmp/cockroach && \
    curl https://binaries.cockroachdb.com/cockroach-v19.2.4.linux-musl-amd64.tgz | \
    tar -xzf - && \
    mv /tmp/cockroach/cockroach-v19.2.4.linux-musl-amd64/cockroach /bin/cockroach && \
    rm -rf /tmp/cockroach