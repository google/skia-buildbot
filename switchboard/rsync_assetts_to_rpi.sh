#!/bin/bash
set -e -x

echo Hello!


echo $1

nc -zv localhost 9000
echo $?

ssh -vvv -F /dev/null -i /home/jcgregorio/.ssh/id_ed25519 -p 9000 root@localhost /bin/bash -c uname -a
scp -v -F /dev/null -P 9000 switchboard/hello root@localhost:/tmp/hello
scp -P 9000 hello_over_adb.sh root@127.0.0.1/tmp/hello_over_adb.sh
ssh -p 9000 root@localhost bash -c /tmp/hello_over_adb.sh