FROM debian:testing-slim AS create
RUN mkdir -p /out/out && cd /out/out \
  && echo 'helloworld' > hello.txt \
  && mkdir subdir && echo 'subdirfile' > subdir/subdirfile 
  #&& for f in $(find /out); do touch -a -c -m -t 197001010000 $f; done \
  #&& touch -a -c -m -t 197001010000 /out/out

FROM scratch
COPY --from=create /out /
