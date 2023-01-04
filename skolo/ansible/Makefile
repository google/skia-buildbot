include ../../make/bazel.mk

# If you need to set up authorized_hosts on machines that don't already have an
# authorized_hosts and only password access you should pass the following flags
# to ansible-playbook:
#
#    -f 1 --ask-become-pass --ask-pass
#
# You will need the "-f 1" because ssh over ${SSH_WRAP} with passwords seems to
# fail with a higher number of parallel forks.

install_test_machine_monitor_to_linux_incremental:
	${SSH_WRAP} ansible-playbook ./switchboard/install_test_machine_monitor.yml --limit skia-e-linux-207 --ask-pass

distribute_authorized_keys_to_rpis:
	${SSH_WRAP} ansible-playbook ./switchboard/update-authorized-keys.yml --limit rpis

# Ignore "[E701] No 'galaxy_info' found" linter warnings."
# Ignore "[E301] Commands should not change things if nothing needs doing"
#    301 is ignored on playbook validation because there are known
#    cases where we need to do that.
validate:
	# Validate playbooks.
	ansible-lint  -x 701,301 -p switchboard/*.yml
	# Validate roles.
	ansible-lint  -x 701     -p switchboard/roles/

update_ssh_gce_config:
	$(BAZEL) run --config=mayberemote --run_under="cd $$PWD &&" //skolo/go/update_ssh_gce_config -- ssh.cfg
