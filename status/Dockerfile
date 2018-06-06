FROM node:8-alpine

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia && \
    apk update && \
    apk add --no-cache git make ca-certificates

COPY . /

USER skia

ENTRYPOINT ["/status"]
CMD ["--logtostderr", "--port=:8000", "--prom_port=:20000", "--workdir=/mnt/pd0/status_workdir", "--use_metadata=true", "--host=status.skia.org", "--resources_dir=/usr/local/share/status", "--capacity_recalculate_interval=30m", "--task_db_url=https://task-scheduler.skia.org/db/"]
