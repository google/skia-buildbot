ARG base
FROM $base
RUN apk add openjdk8
RUN gcloud components install \
    beta \
    cbt \
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
COPY ./run_in_docker.sh /bin/run_emulators.sh
COPY ./healthcheck.sh /bin/healthcheck.sh
ENTRYPOINT [ "/bin/run_emulators.sh" ]
HEALTHCHECK --interval=5s --timeout=60s --start-period=5s --retries=10 CMD [ "/bin/healthcheck.sh" ]
