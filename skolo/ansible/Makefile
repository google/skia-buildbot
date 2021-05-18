# Push a fresh copy of 'authorized_hosts' to the jumphosts.
push_authorized_keys_to_jumphosts:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=jumphosts

# A pre-trial of pushing a fresh copy of 'authorized_hosts' to the RPis on rack4.
push_authorized_keys_to_rack4_rpis_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis_incremental_rollout

push_authorized_keys_to_rack4_rpis_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis

push_authorized_keys_to_rack1_rpis_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack1_rpis_incremental_rollout -f 1 --ask-pass

push_authorized_keys_to_rack1_rpis_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack1_rpis -f 1 --ask-pass

# If you need to set up authorized_hosts on machines that don't already have an
# authorized_hosts and only password access you should pass the following flags
# to ansible-playbook:
#
#    -f 1 --ask-become-pass --ask-pass
#
# You will need the "-f 1" because ssh over ${SSH_WRAP} with passwords seems to
# fail with a higher number of parallel forks.

add_chrome_bot_to_sudoers_rpis_rack4:
	${SSH_WRAP} ansible-playbook add-chrome-bot-to-sudoers.yaml --extra-vars variable_hosts=rack4_rpis --ask-become-pass

get_rack4_rpi_k3s_versions:
	${SSH_WRAP} ansible-playbook rack4/get-k3s-version.yaml --extra-vars variable_hosts=rack4_rpis