serve:
	docker run  -ti -p 1313:1313 -v ~/projects/skia/site2:/docsy/content -v /tmp/output/:/docsy/public  docsy:latest

build:
	docker run --entrypoint=/build.sh  -ti -p 1313:1313 -v ~/projects/skia/site2:/docsy/content -v /tmp/output/:/docsy/public  docsy:latest