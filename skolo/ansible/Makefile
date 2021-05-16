# Push a fresh copy of 'authorized_hosts' to the jumphosts.
push_authorized_keys_to_jumphosts:
	runlocalssh ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=jumphosts

# A pre-trial of pushing a fresh copy of 'authorized_hosts' to the RPis on rack4.
push_authorized_keys_to_rack4_rpis_incremental:
	runlocalssh ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis_incremental_rollout

push_authorized_keys_to_rack4_rpis_full:
	runlocalssh ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis

# If you need to set up authorized_hosts on machines that don't already have an
# authorized_hosts and only password access you should pass the following flags
# to ansible-playbook:
#
#    -f 1 --ask-become-pass --ask-pass
#
# You will need the "-f 1" because ssh over runlocalssh with passwords seems to
# fail with a higher number of parallel forks.

get_rack4_rpi_k3s_versions:
	runlocalssh ansible-playbook rack4/get-k3s-version.yaml

who:
	ANSIBLE_TIMEOUT=60 runlocalssh ansible-playbook rack4/whoami.yaml

rack4-push_authorized_keys:
	# Prime our connection to the rack4 jumphost, which will get reused by ansible.
	runlocalssh ssh rack4 whoami
	ANSIBLE_TIMEOUT=60 runlocalssh ansible-playbook rack4/setup-access.yaml --ask-pass --ask-become-pass