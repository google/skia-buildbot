# jumphost-lighttpd

Installs and runs the lighttpd web server so that `/home/chrome-bot/www`
is served via web server on port 3333.

This will allow Ansible scripts to copy of file to machines by first copying them to the jumphost, and then each machine can pull the files
from the jumphost, which will save bandwidth.

This also works around a current issue where `scp` can't copy files
over a proxy to a Windows machine.
