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
    |   (nginx)   |    ...   |   (nginx)   |
    +------+------+          +------+------+
           |                        |
       +---+-------------+----------+-----+
       v                 v                v
  +---------+        +---------+     +-----------+
  |skia perf|  ...   |skia gold|     |skia alerts|
  +---------+        +---------+     +-----------+

```

A single static IP is handled by GKE Ingress which handles SSL and then
distributes requests to multiple nginx pods: They, in turn, distribute the calls
to the backend as kubernetes sevices. The certs for all Skia properties are
handled at the GKE Ingress level, see http://go/skia-ssl-cert for more details.

The configuration for the nginx server is broken into four files:

~~~
     k8s/
          nginx.conf        # Base nginx config.
          default.conf      # Root of all the server configs.
          redirects.conf    # All redirect rules go into this file.
          services.conf     # This file is generated.
~~~

The file `default.conf` includes `redirects.conf` and `services.conf` and
itself contains all routes that aren't simple, for example, the Gold routes.

All redirects should be placed in `redirects.conf`.

The contents of `services.conf` are generated directly from metadata in
kubernetes Services.

We need three pieces of information, the domain name, the target service name,
and the port that the service is running on. Consider the following
configuration for `https://perf.skia.org`.

~~~
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
~~~

The parts of the Service spec that are important are the `skia.org.domain`
annotation which specifies the public domain name where this service should be
available. It must be a sub-domain of `.skia.org`. There also must be a
`.spec.ports` with a name of "http" that specifies the port at which the service
is available. Along with the name of the service those values are combined to
produce a nginx server config that looks like:

~~~
#####   perf.skia.org   ###########################
server {
    listen      80;
    server_name perf.skia.org;

    location / {
        proxy_pass http://skiaperf:8000;
        proxy_set_header Host $host;
    }
}
~~~

## FAQ

Q: Why not just use GKE Ingress?

A: GKE Ingress is limited to 50 total rules, which we currently exceed.

Q: Why not use TCP Load Balancing and a custom ingress on GKE?

A: GKE Ingress is the only public facing option available to us.