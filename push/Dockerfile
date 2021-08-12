FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/push"]
CMD ["--local", "--config_filename=/etc/push/skiapush.json5", "--resources_dir=/usr/local/share/push"]
