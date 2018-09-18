FROM google/cloud-sdk:latest

RUN addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia

USER skia:skia

COPY . /

ENTRYPOINT ["/usr/local/bin/repo-sync"]
CMD ["--logtostderr"]
