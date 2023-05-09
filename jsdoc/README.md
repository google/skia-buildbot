# jsdoc

Serves demo pages of important elements.

## Design

jsdoc is a simple program that serves up demo pages for select folders
and serves them over HTTP.

## Debugging Locally

`make release` builds a Docker container and uploads it to [GKE](https://console.cloud.google.com/gcr/images/skia-public/GLOBAL/jsdoc?project=skia-public&gcrImageListsize=50).
One can run one of those containers locally as follows:

    docker run -p 8000:8000 gcr.io/skia-public/jsdoc:2018-06-19T17_38_25Z-username-b442da2-dirty

which will map port 8000 in the container (the HTTP server) to port 8000 on the host,
reachable at [localhost:8000].
