# zone-apply

Application that runs in skia-infra-corp and periodically checks out and applies
the zone files in the skfe directory. This will avoid needing to give a human
the ability to change zone files and allow for self-serve creation of
sub-domains for both the `luci.app` and `skia.org` domains.

## Monitoring and Alerting

The application exports the following metrics which will have alerts
created for them and a Grafana dashboad available at [TBD].

```
liveness_zone_apply_refresh_s{name="zone_apply_refresh",type="liveness"}
```

```
zone_has_error{filename="skfe/luci.app.zone"}
zone_has_error{filename="skfe/skia.org.zone"}
```
