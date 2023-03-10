Fiddle
======

Allows trying out Skia code in the browser.

Running locally
---------------

In two different shells:

    $ make run_local_fiddle

    $ make run_local_fiddler

Fiddler uses scrap exchange, but only for scrap conversion.
To test these functions a local scrapexchange will need
to be started:

    # In //scrap run:
    $ PORT=:8010 PROM_PORT=:20001 make run-local-instance

Then visit http://localhost:8080

fiddler Deployment
------------------

The fiddler image is continuously deployed as new Skia commits come in. See
documentation at [docker_pushes_watcher/README.md](../docker_pushes_watcher/README.md).

If needed, fiddler may be manually deployed as so:

    make push_fiddler_I_am_really_sure

Keep in mind that a deployed dirty image will prevent (by design)
docker-pushes-watcher from automatically pushing new images.
To determine if a fiddler push will be clean or dirty run the
following:

    make release-fiddle

To create a clean image make sure that the workspace is up-to-date
with no modified files.

NOTE: One caveat is that the glibc version on the machine performing
the push needs to be compatible with the version on the image being
used by fiddle. After deployment make sure to run a test fiddle to
ensure successful compilation on fiddle.

fiddle Deployment
-----------------

fiddle must be manually deployed as so:

    make push_fiddle

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
