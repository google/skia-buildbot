alertmanger_webproxy
====================

There are three secrets that alertmanager_webproxy requires.
The first two are releated to sending gmail and are created
using `create-secrets.sh`.

The third secret is the config for chat. To configure
it run `get-chat-config.sh` to download the config file.
Manually edit the resulting `chat_config.txt` then upload
the new config by running `put-chat-config.sh`.

grafana
=======

The grafana.ini file should almost never change, so if it does,
just delete the pod and have kubernetes restart it so the config
gets read.

TODO(jcgregorio) Backup the sqlite database.

Admins
======

Before deploying yaml files with service accounts you need to give yourself
cluster-admin rights:

      kubectl create clusterrolebinding \
        ${USER}-cluster-admin-binding \
        --clusterrole=cluster-admin \
        --user=${USER}@google.com

