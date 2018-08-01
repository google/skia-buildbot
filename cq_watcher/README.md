CQ Watcher
==========

The CQ Watcher monitors Gerrit CLs in the Skia CQ and pumps the results into
InfluxDB.

Debugging Locally
-----------------

`make release` builds a Docker container and uploads it to [GKE](https://console.cloud.google.com/gcr/images/skia-public/GLOBAL/cq_watcher?project=skia-public&gcrImageListsize=50).
One can run one of those containers locally as follows:

    docker run gcr.io/skia-public/cq_watcher:2018-06-19T17_38_25Z-username-b442da2-dirty

If you built locally then you can run the latest with:

    docker run cq_watcher
