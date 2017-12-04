#!/bin/bash 

sudo apt-get install python-pip python-dev python-virtualenv
virtualenv --system-site-packages ~/jupyter
source ~/jupyter/bin/activate
pip install --upgrade pip
pip install --upgrade jupyter
pip install --upgrade notebook scipy pandas matplotlib
jupyter notebook
