# zone-apply

Application that runs in skia-infra-corp and periodically checks out and applies
the zone files in the skfe directory. This will avoid needing to give a human
the ability to change zone files and allow for self-serve creation of
sub-domains for both the `luci.app` and `skia.org` domains.

## Monitoring and Alerting

The application exports the following metrics which will have alerts
created for them and a [Grafana dashboard](https://grafana2.skia.org/d/mHshU9sIk/zone-apply?orgId=1).

```
liveness_zone_apply_refresh_s{name="zone_apply_refresh",type="liveness"}
```

```
zone_has_error{filename="skfe/luci.app.zone"}
zone_has_error{filename="skfe/skia.org.zone"}
```

## Permissions

Note that the workload service account,
`zone-apply@skia-infra-corp.iam.gserviceaccount.com` needs `roles/dns.admin` for
the project that contains the zones, in this case that's `skia-public`.