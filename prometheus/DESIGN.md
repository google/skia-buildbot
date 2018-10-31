Prometheus
==========

Prometheus monitoring and AlertManager.


Prometheus doesn't handle authentication, so we run it behind
'proxy_with_auth' that enforce being logged in to a restricted set of domains.

```
    skfe
     |
     +--------+
              |
              V
             prom_proxy_with_auth
               |
               V
             prometheus
```

Metrics from the skolo are brought in via federation. An instance of
Prometheus runs on skia-jumphost and collects metrics and then those metrics
are gathered by the Prometheus instance on skia-prom by using the federation.
The connection between skia-prom and skia-jumphost is over ssh port forwarding
initiated via gcloud compute ssh.

```
  skia-prom
    |
    +-> prometheus
         |
         |
         | [ssh reverse port forwarding]
         |
         V
       skia-jumphost
         |
         +-> prometheus

```

Push metrics are gathered in a similar manner, with port forwarding from
skia-jumphost to the pushgateway running on skia-prom.

```
  skia-prom
    |
    +-> pushgateway
         ^
         |
         | [ssh port forwarding]
         |
         |
       skia-jumphost

```
