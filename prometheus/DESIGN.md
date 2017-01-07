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
