# If you need to set up authorized_hosts on machines that don't already have an
# authorized_hosts and only password access you should pass the following flags
# to ansible-playbook:
#
#    -f 1 --ask-become-pass --ask-pass
#
# You will need the "-f 1" because ssh over ${SSH_WRAP} with passwords seems to
# fail with a higher number of parallel forks.

# Push a fresh copy of 'authorized_hosts' to the jumphosts.
push_authorized_keys_to_jumphosts:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=jumphosts

push_authorized_keys_to_routers:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_router.yml --extra-vars variable_hosts=routers -f 1

# Rack 1
push_authorized_keys_to_rack1_rpis_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack1_rpis_incremental_rollout -f 1 --ask-pass

push_authorized_keys_to_rack1_rpis_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack1_rpis -f 1 --ask-pass

# Rack 2
push_authorized_keys_to_rack2_linux_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack2_linux -l skia-e-linux-207

push_authorized_keys_to_rack2_linux_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack2_linux

push_authorized_keys_to_rack2_mac_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_mac.yml --extra-vars variable_hosts=rack2_mac -l skia-e-mac-220 --ask-pass

push_authorized_keys_to_rack2_mac_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_mac.yml --extra-vars variable_hosts=rack2_mac -f 1 --ask-pass

push_authorized_keys_to_rack2_win_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_win.yml --extra-vars variable_hosts=rack2_win -l skia-e-win-201 --ask-pass

push_authorized_keys_to_rack2_win_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_win.yml --extra-vars variable_hosts=rack2_win -f 1 --ask-pass


# Rack 3
push_authorized_keys_to_rack3_linux_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack3_linux -l skia-e-linux-307 --ask-pass

push_authorized_keys_to_rack3_linux_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack3_linux -f 1 --ask-pass

push_authorized_keys_to_rack3_mac_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_mac.yml --extra-vars variable_hosts=rack3_mac -l skia-e-mac-330 --ask-pass

push_authorized_keys_to_rack3_mac_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_mac.yml --extra-vars variable_hosts=rack3_mac -f 1 --ask-pass

push_authorized_keys_to_rack3_win_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_win.yml --extra-vars variable_hosts=rack3_win -l skia-e-win-302

push_authorized_keys_to_rack3_win_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_win.yml --extra-vars variable_hosts=rack3_win -f 1 --ask-pass


# Rack 4
push_authorized_keys_to_rack4_rpis_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis_incremental_rollout

push_authorized_keys_to_rack4_rpis_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack4_rpis

add_chrome_bot_to_sudoers_rpis_rack4:
	${SSH_WRAP} ansible-playbook add-chrome-bot-to-sudoers.yaml --extra-vars variable_hosts=rack4_rpis --ask-become-pass

get_rack4_rpi_k3s_versions:
	${SSH_WRAP} ansible-playbook rack4/get-k3s-version.yaml --extra-vars variable_hosts=rack4_rpis

# Rack 5
push_authorized_keys_to_rack5_linux_incremental:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack5_linux -l skia-i-gapid-001 --ask-pass

push_authorized_keys_to_rack5_linux_full:
	${SSH_WRAP} ansible-playbook push_authorized_keys_to_linux.yml --extra-vars variable_hosts=rack5_linux -f 1 --ask-pass
