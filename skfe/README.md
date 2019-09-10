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
handles at the GKE Ingress level, see http://go/skia-ssl-cert for more details.

## FAQ

Q: Why not just use GKE Ingress?

A: GKE Ingress is limited to 50 total rules, which we currently exceed.

Q: Why not use TCP Load Balancing and a custom ingress on GKE?

A: GKE Ingress is the only public facing option available to us.