#! /bin/bash

cd $HOME
sudo apt-get update
sudo apt remove cmdtest # https://github.com/yarnpkg/yarn/issues/2821
wget https://nodejs.org/dist/v8.10.0/node-v8.10.0-linux-x64.tar.xz
tar -xf node-v8.10.0-linux-x64.tar.xz
mv node-v8.10.0-linux-x64 node
curl -o- -L https://yarnpkg.com/install.sh | bash
rm *.xz
echo 'export PATH="$HOME/node/bin:$HOME/.yarn/bin:$HOME/.config/yarn/global/node_modules/.bin:$PATH"' >> ~/.bashrc
