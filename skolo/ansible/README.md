This works, but occasionally gets in a bad place and I have to run:

        $ killall corp-ssh-helper

Runs might fail for a small number of hosts, you can re-run a script for a
specific host by passing -l (hostname) to the ansible-playbook command.
