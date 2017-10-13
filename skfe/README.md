SKFE - The Skia Frontend
========================

A single unified web frontend for all the Skia properties.

~~~~

         +--------------------------+
         |  Google Compute Engine   |
         |  Protocol Load Balancing |
         +----+------------------+--+
              |                  |
              |                  |
              v                  v
    +-------------+          +-------------+
    | skia-skfe-1 |          | skia-skfe-2 |
    |   (nginx)   |          |   (nginx)   |
    +------+------+          +------+------+
           |                        |
       +---+-------------+----------+-----+
       v                 v                v
  +---------+        +---------+     +-----------+
  |skia perf|  ...   |skia gold|     |skia alerts|
  +---------+        +---------+     +-----------+

~~~~

A single static IP is handled by GCE Network load balancing
which distributes requests to two NGINX servers:
   * skia-skfe-1
   * skia-skfe-2

They, in turn, handle SSL and then distribute the calls to the backends:
skiaperf, skiagold, skiadocs, skiapush, skiaalerts, etc.
For the load balancing setup, see the [cloud console page](https://console.cloud.google.com/net-services/loadbalancing/loadBalancers/list?project=google.com:skia-buildbots).
The forwarding rule is named `skfe-pool-rule`, and the target pool is
`skfe-pool`. To increase the capacity just add more skia-skfe-N servers
to the target pool.

All nginx servers will use certpoller to provide certs, and will
all handle \*.skia.org traffic with a wildcard cert. The certs
are stored in GCE Project Level Metadata and are updated via
certpoller. The configurations for the nginx servers are handled
as a push package that contains the nginx proxy rules for each
backend.
