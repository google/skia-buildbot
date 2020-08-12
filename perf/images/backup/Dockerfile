FROM gcr.io/google.com/cloudsdktool/cloud-sdk:latest

RUN apt install wget

RUN wget -qO- https://binaries.cockroachdb.com/cockroach-v20.1.3.linux-amd64.tgz | tar  xvz

RUN cp -i cockroach-v20.1.3.linux-amd64/cockroach /usr/local/bin/

COPY . /

ENTRYPOINT ["/backup.sh"]