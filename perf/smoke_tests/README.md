Tests perf... are experimental tests.

## To run a proper test, do the following:

Remove issue_tracker_config from v8-internal.json config.
Go to buildbot/perf directory, and run the following:

```bash
make run-auth-proxy-before-demo-instance
./run_with_spanner.sh p=skia-infra-corp i=tfgen-spanid-20241205020733610 \
d=v8_int config=./configs/spanner/v8-internal.json
bazel test //perf/smoke_tests:regression_page_nodejs_test
```

TODO(mordeckimarcin) We will look into integrating this into
the `run_perfserver.sh` script in the future.

## To see the execution, do the following:

Start your CRD and from there, invoke the test, adding:

```bash
DISPLAY=:20 bazel test --test_output=streamed \
//perf/smoke_tests:regression_page_nodejs_test --test_env=DEBUG_VIA_CRD=true
```

By default, tests are executed against a locally hosted instance with
auth-proxy enabled - usually, the address is `http://localhost:8003`.

You can change instance the tests are run against by adding:

```bash
--test_env=PERF_BASE_URL=the_instance_you_wish_to_test
```
