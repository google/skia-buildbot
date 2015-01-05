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
which distributes reqeusts to two NGINX servers:
   * skia-skfe-1
   * skia-skfe-2

They, in turn, handle SSL and then distrubute the calls to the backends:
skiaperf, skiagold, skiadocs, skiapush, skiaalerts, etc.
For the load balancing setup, see the [cloud console page](https://console.developers.google.com/project/31977622648/compute/loadBalancing/forwardingRules/forwardingRulesDetail/regions/us-central1/forwardingRules/skfe-rule).
The forwarding rule is name `skfe-rule`, and the target pool is `skfe-pool`.
To increase the capacity just add more skia-skfe-N servers to the
target pool.

All nginx servers will use certpoller to provide certs, and will
all handle \*.skia.org traffic with a wildcard cert. The certs
are stored in GCE Project Level Metadata and are updated via
certpoller. The configurations for the nginx servers are handled
as a push package that contains the nginx proxy rules for each
backend.
