run:
	 bazel test :run_test_on_leased_rpi --crosstool_top=@androidndk//:default_crosstool --cpu=armeabi-v7a --host_crosstool_top=@bazel_tools//tools/cpp:toolchain --force_pic --test_output=streamed --cache_test_results=no


# Must run this outside first:
#    kubectl --logtostderr port-forward switch-pod-1 9000
switchboard:
	bazel test :run_test_on_leased_rpi_via_switchboard --crosstool_top=@androidndk//:default_crosstool --cpu=armeabi-v7a --host_crosstool_top=@bazel_tools//tools/cpp:toolchain --force_pic --test_output=streamed --cache_test_results=no

release: build
	./images/switchboard/release

switch-pod-release:
	go install ./go/...
	./images/switch-pod/release