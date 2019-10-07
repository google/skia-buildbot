FROM gcr.io/skia-public/base-cipd:testing-slim

USER skia:skia

COPY . /

RUN cipd ensure -root=${CIPD_ROOT} -ensure-file /usr/local/share/task-scheduler/cipd.ensure

ENV PATH="${CIPD_ROOT}/git/bin:${CIPD_ROOT}/python/bin:${CIPD_ROOT}/python:${CIPD_ROOT}/luci-go:${PATH}"

ENTRYPOINT ["/usr/local/bin/task-scheduler"]
