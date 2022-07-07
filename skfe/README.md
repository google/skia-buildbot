# SKFE - The Skia Frontend

A single unified web frontend for all the Skia properties.

```

         +--------------------------+
         |      GKE Ingress         |
         +----+------------------+--+
              |                  |
              |                  |
              v                  v
    +-------------+          +-------------+
    |   (envoy)   |    ...   |   (envoy)   |
    +------+------+          +------+------+
           |                        |
       +---+-------------+----------+-----+
       v                 v                v
  +---------+        +---------+     +-----------+
  |skia perf|  ...   |skia gold|     |skia alerts|
  +---------+        +---------+     +-----------+

```

A single static IP is handled by GKE Ingress which handles SSL and then
distributes requests to multiple envoy pods: They, in turn, distribute the calls
to the backend as kubernetes sevices. The certs for all Skia properties are
handled at the GKE Ingress level, see http://go/skia-ssl-cert for more details.

The configuration for the envoy server comes from `envoy-starter.json` and
metadata annotations on services running in the skia-public k8s cluster.

The generation of the configuration involves three files, two of which are
auto-generated:

    envoy-starter.json
    simple.json
    computed.json

The `simple.json` and `computed.json` are generated files.

The `envoy-starter.json` file contains our liveness handling, all redirects, and
complicated routes for services like Gold. It is a hand-written file.

The contents of `simple.json` are generated directly from metadata in
kubernetes Services.

We need three pieces of information, the domain name, the target service name,
and the port that the service is running on. Consider the following
configuration for `https://perf.skia.org`.

```
apiVersion: v1
kind: Service
metadata:
  labels:
    app: skiaperf
  name: skiaperf
  annotations:
    beta.cloud.google.com/backend-config: '{"ports": {"8000":"skia-default-backendconfig"}}'
    skia.org.domain: perf.skia.org
spec:
  ports:
    - name: metrics
      port: 20000
    - name: http
      port: 8000
  selector:
    app: skiaperf
  type: NodePort
```

The parts of the Service spec that are important are the `skia.org.domain`
annotation which specifies the public domain name where this service should be
available. It must be a sub-domain of `.skia.org`. There also must be a
`.spec.ports` with a name of "http" that specifies the port at which the service
is available. Along with the name of the service those values are combined to
produce the `simple.json` config file. We then run `merge_envoy` to merge
`simple.json` and `envoy-starter.json` into `computed.json` which is then used
in the envoy image. Making this a two step process makes it easier to debug the
output of each step, as Envoy configs are very wordy.

Finally `update_probers` can be run to add all the redirects from
`envoy-starter.json` to `probersk.json5`.

## Pushing

Run

    make

To build the `computed.json` config file which is then reviewed and submitted.

After that file has landed then run:

    make push

to push the `computed.json` file to production.

## Admin

You can reach the Envoy admin interface by

    kubectl port-forward <envoy-skia-org pod name>  9000:9000

And then visit

    http://localhost:9000

The admin interface also provides the metrics endpoint to prometheus.

## DNS

Our DNS Zone file is checked in here as [skia.org.zone](./skia.org.zone). See
that file for instructions on how to update our DNS records.

## FAQ

Q: Why not just use GKE Ingress?

A: GKE Ingress is limited to 50 total rules, which we currently exceed.

Q: Why not use TCP Load Balancing and a custom ingress on GKE?

A: GKE Ingress is the only public facing option available to us.
