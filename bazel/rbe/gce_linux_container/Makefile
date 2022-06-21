# We don't tag this container because we expect it to change very infrequently, thus the utility
# and extra complexity of generating unique tags is very low.

NAME = gcr.io/skia-public/infra-rbe-linux

.PHONY: build
build:
	docker build -t ${NAME} .

.PHONY: run
run: build
	docker run -it ${NAME} /bin/bash

.PHONY: push
push: build
	docker push ${NAME}
