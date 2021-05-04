push_authorized_keys_to_jumphosts:
	runlocalssh ansible-playbook push_authorized_keys_to_jumphosts.yml

rack4_rpi_setup_access:
	runlocalssh ansible-playbook --ask-pass --ask-become-pass rack4/setup-access.yaml

get_rack4_rpi_k3s_versions:
	runlocalssh ansible-playbook rack4/get-k3s-version.yaml