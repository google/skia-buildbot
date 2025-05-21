#!/bin/bash
#
# Utilities for accessing the skolo.
# Should be sourced from $HOME/.bashrc

alias skolo_internal='ssh chrome-bot@100.115.95.131'

# Sets up port-forwarding to the Router.
# Router ports start at 9000.
alias skolo_rack5_router='google-chrome https://localhost:9004; ssh -L 9004:192.168.1.1:443 rack5'
