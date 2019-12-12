
# Args do not yet expand in COPY: https://github.com/docker/cli/issues/996
# So using workaround with Args mentioned in the above issue.
ARG SKIA_WASM_IMAGE_NAME=skia-wasm-release
ARG SKIA_WASM_IMAGE_TAG=prod
ARG SKIA_IMAGE_NAME=skia-release
ARG SKIA_IMAGE_TAG=prod
FROM gcr.io/skia-public/${SKIA_WASM_IMAGE_NAME}:${SKIA_WASM_IMAGE_TAG} as SKIA_WASM_IMAGE
FROM gcr.io/skia-public/${SKIA_IMAGE_NAME}:${SKIA_IMAGE_TAG} as SKIA_IMAGE

FROM gcr.io/skia-public/basealpine:3.8

COPY . /

COPY --from=SKIA_WASM_IMAGE \
  /tmp/debugger/debugger.wasm \
  /usr/local/share/debugger-assets/res/
COPY --from=SKIA_WASM_IMAGE \
  /tmp/debugger/debugger.js \
  /usr/local/share/debugger-assets/res/js/

USER skia

COPY --from=SKIA_IMAGE /tmp/skia/skia/VERSION /etc/skia-prod/VERSION

ENTRYPOINT ["/usr/local/bin/debugger-assets"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger-assets"]
