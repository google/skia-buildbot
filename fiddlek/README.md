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

The fiddler image is continuously deployed as GCP Container Builder succeeds
in building new gcr.io/skia-public/fiddler images. The app that does the
deployment is infra/kube/go/continuous-deploy.
