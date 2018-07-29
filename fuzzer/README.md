Fuzzer
======

When the instance is created, afl-fuzz is downloaded from `gs://skia-fuzzer/afl-mirror/afl-[version].tgz`
To update afl-versions, download the .tgz file from [http://lcamtuf.coredump.cx/afl/releases/?O=D],
upload it to that location and make it publicly shared.

AFL-fuzz requires that core dumps be handled normally and not notify external entities.  As such, this may need to be run as root (sudo su):
`echo core >/proc/sys/kernel/core_pattern`


AFL-fuzz notes:
Try minimizing the test cases first for better performance:

```
#This will lock the test case in to those that run in under a second
FUZZ_INPUT="$HOME/SKP/small"
FUZZ_SAMPLES="$HOME/SKP/minimized"
./afl-cmin -i $FUZZ_INPUT -o $FUZZ_SAMPLES -m 1000 -t 1000 -- $SKIA_ROOT/out/Release/dm --src skp --skps @@ --config 8888
```


When deployed to production, the params in fuzzer-be.service and fuzzer-fe.service can be tuned via
experimentation for optimal performance.