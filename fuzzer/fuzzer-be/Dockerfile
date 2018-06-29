# Dockerfile for running fuzzer backend (aka fuzzer-be)
# This involves compiling Skia and running afl-fuzz
FROM launcher.gcr.io/google/clang-debian9 AS build

# Things we need to build Skia
RUN apt-get update && apt-get upgrade -y && apt-get install -y \
  git \
  python \
  curl \
  make \
  build-essential \
  libfontconfig-dev \
  libgl1-mesa-dev \
  libglu1-mesa-dev \
  && groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia

RUN mkdir -p -m 0755 /opt/afl && \
  mkdir -p -m 0777 /opt/depot_tools

ENV AFL_VERSION=2.51b CC=/usr/local/bin/clang CXX=/usr/local/bin/clang++ AFL_SKIP_CPUFREQ=1

ADD https://storage.googleapis.com/skia-fuzzer/afl-mirror/afl-$AFL_VERSION.tgz /tmp/afl.tgz

# Make afl-fuzz
RUN tar -C /opt/afl -zxf /tmp/afl.tgz --strip=1 "afl-"$AFL_VERSION && \
  rm /tmp/afl.tgz && \
  cd /opt/afl && \
  make
# Can't build afl-clang-fast because llvm-config not on image
# TODO(kjlubick): Do that if we need the performance boost

COPY . /

RUN mkdir -m 0777 /mnt/fuzzing/

USER skia

RUN git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git' /opt/depot_tools

# This is different from how we normally do things (normally we do ENTRYPOINT and CMD)
# because we want these flags to be set for all fuzzer instances. What is set in the
# Kubernetes config will just be the unique settings (e.g. what fuzzers to run)
ENTRYPOINT ["/fuzzer-be", "--logtostderr", "--skia_root=/mnt/fuzzing/skia-be", \
            "--clang_path=/usr/local/bin/clang", "--clang_p_p_path=/usr/local/bin/clang++", \
            "--depot_tools_path=/opt/depot_tools", "--afl_root=/opt/afl", \
            "--afl_output_path=/mnt/fuzzing/afl-out", "--fuzz_samples=/mnt/fuzzing/samples", \
            "--generator_working_dir=/mnt/fuzzing/generator-wd", \
            "--aggregator_working_dir=/mnt/fuzzing/aggregator-wd", \
            "--executable_cache_path=/mnt/fuzzing/executable_cache", \
            "--fuzz_path=/mnt/fuzzing/fuzzes", \
            "--status_period=10s", "--analysis_timeout=5s"]