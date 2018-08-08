alert-manager
=============

Testing Locally
---------------

To run a local copy of Prometheus that will emit an alert
you can build and run the following container:

```
docker build -t my-prom ./testlocal
docker run -p 9090:9090 --net="host" my-prom
```

Make sure alert-to-pubsub is running locally at port 8000.

Datastore
---------

Indices are in ../ds/index-skia-public.yaml and can be created using:

```
gcloud datastore create-indexes index-skia-public.yaml
```
