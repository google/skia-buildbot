#! /bin/bash

set -x

# Remove the default host and install the certs then restart nginx.
# Note, if one of the next two lines fail, nginx will continue to run with the
# previous configuation, which makes it fault silent but available.
rm -f /etc/nginx/sites-enabled/default
/usr/local/bin/certpoller --logtostderr skia-org-pem skia-org-key dh-params-pem
systemctl daemon-reload
systemctl restart nginx
