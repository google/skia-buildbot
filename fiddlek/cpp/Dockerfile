# Ran the following on May 23 2024
# docker pull launcher.gcr.io/google/debian11:latest && docker images --digests | grep debian11
FROM launcher.gcr.io/google/debian11@sha256:b8c3083f68a12baaaa9a2470672069b5c364d3a9a5d8ca396fd69ee029f53c38

# The versions were added after seeing what was available when this image was created on May 23 2024
# Specifying the versions makes this Docker container comply with SLSA level 1.
RUN apt-get update && \
    apt-get install -y --no-install-recommends  \
           ca-certificates=20210119 \
           build-essential=12.9 \
           clang=1:11.0-51+nmu5

COPY . /tmp/

WORKDIR /tmp
RUN clang fiddle_secwrap.cpp -lstdc++ -o fiddle_secwrap

# compile test executables
RUN mkdir /tmp/tests
RUN clang success.cpp -lstdc++ -o ./tests/success
RUN clang fail_socket.cpp -o ./tests/fail_socket
RUN clang fail_execve.cpp -o ./tests/fail_execve
RUN clang fail_link.cpp -o ./tests/fail_link
RUN clang fail_rename.cpp -o ./tests/fail_rename
RUN clang fail_mknod.cpp -o ./tests/fail_mknod

# Make sure this binary runs on its own...
RUN ./tests/success
# ...and with the wrapper
RUN ./fiddle_secwrap ./tests/success

# If any of these don't work, the security wrapper is not blocking things correctly.
RUN ./fiddle_secwrap ./tests/fail_socket; test $? -eq 1
RUN ./fiddle_secwrap ./tests/fail_execve; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_link; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_rename; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_mknod; test $? -eq 255