run:
	 bazel test :run_test_on_leased_rpi --crosstool_top=@androidndk//:default_crosstool --cpu=armeabi-v7a --host_crosstool_top=@bazel_tools//tools/cpp:toolchain --force_pic --test_output=streamed --cache_test_results=no


switchboard:
	 bazel test :test_on_remote_pixel --crosstool_top=@androidndk//:default_crosstool --cpu=armeabi-v7a --host_crosstool_top=@bazel_tools//tools/cpp:toolchain --force_pic --test_output=streamed --cache_test_results=no

release: build
	./images/switchboard/release

switch-pod-release:
	./images/switch-pod/release