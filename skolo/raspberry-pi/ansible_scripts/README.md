Ansible cheat sheet
===================

The playbooks here are examples for doing large scale maintanence on the rpi fleet.

See https://docs.google.com/document/d/1o07eSiEnzDS0D90HRn_fIWEGOUqmPx3w1d-LP-_MdUQ/edit for more information.

# Run on one rpi
ansible-playbook -i rpi-hosts -l 192.168.1.215 print-hostname.yml -vv

# Run on all rpis listed in rpi-hosts
ansible-playbook -i rpi-hosts print-hostname.yml -vv
