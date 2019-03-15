#!/bin/bash
#
# Utilities for accessing the skolo.
# Should be sourced from $HOME/.bashrc

alias skolo_internal='ssh chrome-bot@100.115.95.131'
alias skolo_linux='ssh chrome-bot@100.115.95.130'
alias skolo_rpi='ssh chrome-bot@100.115.95.143'
alias skolo_rpi2='ssh chrome-bot@100.115.95.135'
alias skolo_win='ssh chrome-bot@100.115.95.132'
alias skolo_win2='ssh chrome-bot@100.115.95.133'
alias skolo_win3='ssh chrome-bot@100.115.95.134'

# Sets up port-forwarding to the Router.
alias skolo_internal_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.131'
alias skolo_linux_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.130'
alias skolo_rpi_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.143'
alias skolo_rpi2_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.135'
alias skolo_win_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.132'
alias skolo_win2_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.133'
alias skolo_win3_router='google-chrome https://localhost:8888; ssh -L 8888:192.168.1.1:443 chrome-bot@100.115.95.134'
