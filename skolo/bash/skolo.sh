#!/bin/bash
#
# Utilities for accessing the skolo.
# Should be sourced from $HOME/.bashrc

alias skolo_internal='ssh chrome-bot@100.115.95.131'
alias skolo_rack1='ssh chrome-bot@100.115.95.143'
alias skolo_rack2='ssh chrome-bot@100.115.95.133'
alias skolo_rack3='ssh chrome-bot@100.115.95.134'
alias skolo_rack4='ssh chrome-bot@100.115.95.135'
alias skolo_rpi='ssh chrome-bot@100.115.95.143'
alias skolo_rpi2='ssh chrome-bot@100.115.95.135'
alias skolo_win2='ssh chrome-bot@100.115.95.133'
alias skolo_win3='ssh chrome-bot@100.115.95.134'

# Sets up port-forwarding to the Router.
# Router ports start at 9000.
alias skolo_rack1_router='google-chrome https://localhost:9000; ssh -L 9000:192.168.1.1:443 rack1'
alias skolo_rack2_router='google-chrome https://localhost:9001; ssh -L 9001:192.168.1.1:443 rack2'
alias skolo_rack3_router='google-chrome https://localhost:9002; ssh -L 9002:192.168.1.1:443 rack3'
alias skolo_rack4_router='google-chrome https://localhost:9003; ssh -L 9003:192.168.1.1:443 rack4'
alias skolo_rack5_router='google-chrome https://localhost:9004; ssh -L 9004:192.168.1.1:443 rack5'


# Shelf ports start at 7000, and the second digit is the rack number, the last number is the shelf.
alias skolo_rack1_shelf2_switch='google-chrome https://localhost:7101; ssh -L 7101:rack1-shelf1-poe-switch:443 rack1'
alias skolo_rack1_shelf2_switch='google-chrome https://localhost:7102; ssh -L 7102:rack1-shelf2-poe-switch:443 rack1'

alias skolo_rack4_shelf1_switch='google-chrome https://localhost:7401; ssh -L 7401:rack4-shelf1-poe-switch:443 rack4'
alias skolo_rack4_shelf2_switch='google-chrome https://localhost:7402; ssh -L 7402:rack4-shelf2-poe-switch:443 rack4'
