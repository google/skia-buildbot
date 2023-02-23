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

The configuration for the envoy server is generated in the [k8s-config](https://skia.googlesource.com/k8s-config/+/refs/heads/main/)
repo from `//skfe/envoy-starter.json` and metadata annotations on service files in the
three k8s clusters.

The generation of the configuration involves four files, the last three are
auto-generated:

    envoy-starter.json
    skia-infra-corp.json
    skia-infra-public.json
    skia-public.json

The [`envoy-starter.json`](https://skia.googlesource.com/k8s-config/+/refs/heads/main/skfe/envoy-starter.json)
file contains our liveness handling, all redirects, and
complicated routes for services like Gold. It is a hand-written file.

The contents of a cluster config file, e.g.
[`skia-public.json`](https://skia.googlesource.com/k8s-config/+/refs/heads/main/skfe/skia-public.json)
are generated from metadata in kubernetes Services and this `envoy-starter.json` file.

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
is available.

## Admin

You can reach the Envoy admin interface by

    kubectl port-forward <envoy-skia-org pod name>  9000:9000

And then visit

    http://localhost:9000

The admin interface also provides the metrics endpoint to prometheus.

## Deploying Changes

1. Make changes in the [`k8s-config`](https://skia.googlesource.com/k8s-config/+/refs/heads/main/) repo.
   For example, add a new service's .yaml file in the correct cluster subfolder.
2. Run `make` at the top level of the `k8s-config` repo.
3. Get the CL reviewed and checked in. It will automatically be rolled out.

## DNS

Our DNS Zone file is checked in here as [skia.org.zone](./skia.org.zone). See
that file for instructions on how to update our DNS records.

## FAQ

Q: Why not just use GKE Ingress?

A: GKE Ingress is limited to 50 total rules, which we currently exceed.

Q: Why not use TCP Load Balancing and a custom ingress on GKE?

A: GKE Ingress is the only public facing option available to us.
