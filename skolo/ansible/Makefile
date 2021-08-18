# If you need to set up authorized_hosts on machines that don't already have an
# authorized_hosts and only password access you should pass the following flags
# to ansible-playbook:
#
#    -f 1 --ask-become-pass --ask-pass
#
# You will need the "-f 1" because ssh over ${SSH_WRAP} with passwords seems to
# fail with a higher number of parallel forks.

install_test_machine_monitor_to_linux_incremental:
	${SSH_WRAP} ansible-playbook ./switchboard/install_test_machine_monitor.yml --extra-vars variable_hosts=rack2_linux -l skia-e-linux-207 --ask-pass

distribute_authorized_keys_to_rpis:
	${SSH_WRAP} ansible-playbook ./switchboard/update-authorized-keys.yml --extra-vars variable_hosts=rpi
