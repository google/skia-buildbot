VERSION=v1
publish:
	# We want a statically linked binary so as not to have any GLIBC or other dymamic dependencies
	# to worry about.
	bazelisk build :open_ar --@rules_go//go/config:static
	gsutil cp ../../_bazel_bin/bazel/open_ar/open_ar_/open_ar \
		gs://skia-cdn/bazel/open_ar_${VERSION}
	sha256sum ../../_bazel_bin/bazel/open_ar/open_ar_/open_ar
