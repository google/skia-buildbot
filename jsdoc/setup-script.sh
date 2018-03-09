#! /bin/bash

pushd /home/default

sudo apt-get update
sudo apt remove cmdtest # https://github.com/yarnpkg/yarn/issues/2821
curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add - echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
sudo apt-get update && sudo apt-get install yarn
echo "alias nodejs=node" >> ~/.bashrc
