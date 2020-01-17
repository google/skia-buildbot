FROM gcr.io/skia-public/base-cipd:testing-slim AS git
COPY ./tmp/cipd.ensure /tmp/cipd.ensure
RUN mkdir ${CIPD_ROOT}/cipd_bin_packages && cd ${CIPD_ROOT}/cipd_bin_packages \
  && cipd install $(cat /tmp/cipd.ensure | grep "infra/3pp/tools/git") \
  && cipd install $(cat /tmp/cipd.ensure | grep "infra/tools/git") \
  && cipd install $(cat /tmp/cipd.ensure | grep "infra/tools/luci/git-credential-luci") \
  && git config --global user.name skia \
  && git config --global user.email skia
