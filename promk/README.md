alertmanger_webproxy
====================

There are three secrets that alertmanager_webproxy requires.
The first two are releated to sending gmail and are created
using `create-secrets.sh`.

The third secret is the config for chat. To configure
it run `get-chat-config.sh` to download the config file.
Manually edit the resulting `chat_config.txt` then upload
the new config by running `put-chat-config.sh`.

