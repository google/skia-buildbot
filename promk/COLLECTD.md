collectd.skia.org
=================

Handles POST'd JSON from collectd servers using the write_http plugin.

The collectd service should be configured as such:

    <Plugin write_http>
        <Node "somename">
            URL "https://collectd.skia.org/collectd-post"
            Format "JSON"
       </Node>
    </Plugin>

The service that accepts the POST's is https://github.com/prometheus/collectd_exporter.
