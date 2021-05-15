# Push a fresh copy of 'authorized_hosts' to the jumphosts.
push_authorized_keys_to_jumphosts:
	runlocalssh ansible-playbook push_authorized_keys_to_jumphosts.yml

get_rack4_rpi_k3s_versions:
	runlocalssh ansible-playbook rack4/get-k3s-version.yaml

who:
	ANSIBLE_TIMEOUT=60 runlocalssh ansible-playbook rack4/whoami.yaml

rack4-push_authorized_keys:
	# Prime our connection to the rack4 jumphost, which will get reused by ansible.
	runlocalssh ssh rack4 whoami
	ANSIBLE_TIMEOUT=60 runlocalssh ansible-playbook rack4/setup-access.yaml --ask-pass --ask-become-pass 