Fiddle
======

Allows trying out Skia code in the browser.

Running locally
---------------

To run locally:

    $ make image

Then in two different shells:

    $ make run_local_fiddle

    $ make run_local_fiddler

Then visit http://localhost:8080

Continuous Deployment of fiddler
--------------------------------

The fiddler image is continuously deployed as new Skia commits come in. See
documentation at [docker_pushes_watcher/README.md](../docker_pushes_watcher/README.md).

Node Pool
---------

The fiddler-pool node pool is dedicated to running just fiddler pods. This was
setup because fiddler latency was high for fiddle when run along with many
other pods on 64 core nodes. To create a node pool that is dedicated to a
certain kind of pod you need to label and taint all the nodes in the node
pool, in this case with the same key,value pair:

    reservedFor=fiddler

Using the same key/value pair isn't required, but it keeps it consistent.

Then add a tolerance to the pod description so it can run in the node-pool,
and also add a nodeSelector so that the pods get scheduled into the pool.

    spec:
      nodeSelector:
        reservedFor: fiddler
      tolerations:
        - key: "reservedFor"
          operator: "Equal"
          value: "fiddler"
          effect: "NoSchedule"
