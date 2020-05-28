default: android_compile_fe
	npx webpack --mode=development

serve:
	npx webpack-dev-server --mode=development --watch-poll

watch:
	npx webpack --mode=development --watch

android_compile_fe:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/android_compile_fe

release_fe: android_compile_fe
	npm ci
	npx webpack --mode=production
	./build_fe_release

android_compile_be:
	GOOS=linux go install -v ./go/android_compile_be

release_be: android_compile_be
	./build_be_release

push_fe: release_fe
	pushk android_compile_fe

push_be: release_be
	pushk android_compile_be
