Fuzzer
======



AFL-fuzz notes:
Try minimizing the test cases first for better performance:

```
#This will lock the test case in to those that run in under a second
FUZZ_INPUT="$HOME/SKP/small"
FUZZ_SAMPLES="$HOME/SKP/minimized"
./afl-cmin -i $FUZZ_INPUT -o $FUZZ_SAMPLES -m 1000 -t 1000 -- $SKIA_ROOT/out/Release/dm --src skp --skps @@ --config 8888
```
