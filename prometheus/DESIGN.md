Prometheus
==========

Prometheus monitoring and AlertManager.


Prometheus and AlertManager don't handle authentication, so we run them behind
instances of 'proxy_with_auth' that enforce being logged in to a restricted
set of domains.

In addition, there is webhook_email_proxy, which takes webhook requests from
alertmanager and turns them into emails using the gmail api.


```
    skfe
     |
     +--------+
     |        |
     |        V
     |       prom_proxy_with_auth
     |         |
     |         V
     |       prometheus
     V
    alert_proxy_with_auth
       |
       V
    alertmanager
          |
          V
    webhook_email_proxy
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
