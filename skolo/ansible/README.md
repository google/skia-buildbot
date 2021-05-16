# Skolo Ansible Access

These Ansible scripts need to be run from your desktop machine and not from a
lab computer.

1. First visit http://go/corp-ssh-helper and following the directions there.
2. Append the [ssh.cfg](ssh.cfg) file to your existing `~/.ssh/config` file.
3. Install Ansible: `sudo apt install ansible`.

At this point you should be able to connect to any skolo device. Test this by
trying:

        $ ssh rack4

or

        $ ssh skia-rpi-001

Once that is complete see the Makefile for the actions you can perform via
Ansible.

## Debugging

Occasionally an Ansible run will fail all ssh connections, it appears
corp-ssh-helper gets in a bad place, you can usually fix this by running:

        $ killall corp-ssh-helper

## Tips

Runs might fail for a small number of hosts, you can re-run a script for a
specific host by passing `-l (hostname)` to the `ansible-playbook` command.

Add:

    export DISABLE_PROD_WARNING=1

To you `.bashrc` to stop runlocalssh from spewing warnings.

## Design

Most of the proxy config is done in ssh.cfg, but we add a couple extra args via
ansible.cfg ssh_connection section.
