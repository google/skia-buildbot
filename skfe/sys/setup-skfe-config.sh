#! /bin/bash

set -e
set -x

# Remove the default host and install the certs then restart nginx.
# Note, if one of the next two lines fail, nginx will continue to run with the
# previous configuation, which makes it fault silent but available. 
sudo rm -f /etc/nginx/sites-enabled/default
sudo /usr/local/bin/certpoller --log_dir=/var/log/logserver \
     skia-org-pem skia-org-key
sudo systemctl restart nginx
