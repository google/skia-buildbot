Installing Jupyter
==================

Jupyter and IPython are written in Python and Python installs, particularly on
older Linux distributions, is problematic. This is the most reliable way
to install Jupyter and IPython:

        sudo apt-get install python-pip python-dev python-virtualenv
        virtualenv --system-site-packages ~/jupyter
        source ~/jupyter/bin/activate
        pip install --upgrade pip
        pip install --upgrade jupyter
        pip install --upgrade notebook scipy pandas matplotlib
        jupyter notebook

Once you are done running Jupyter you can deactivate the virtualenv
environment:

        deactivate

Once you have everything installed running will just be:

        source ~/jupyter/bin/activate
        jupyter notebook

Remembering to deactivate the virtualenv environment:

        deactivate
