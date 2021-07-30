# Skolo Ansible Access

These Ansible scripts can be run from a desktop machine and not from a lab
computer.

Visit http://go/skolo-maintenance#heading=h.or4jzu6r2mzn for instructions on how
to set up to run these commands.

## Tips

Runs might fail for a small number of hosts, you can re-run a script for a
specific host by passing `-l (hostname)` to the `ansible-playbook` command.

You can target a machine not in the skolo by referring to it by IP address,
presuming the IP address is in the range 192.168.0.0/16. This is defined in
`hosts.ini` as the `[local]` group of machines.

    $ ansible-playbook ./switchboard/rpi.yml --extra-vars variable_hosts=192.168.1.157

## Notes

See `./group_vars/all.yml` for variables that are defined everywhere.

See `hosts.ini` for all the hosts and groups of hosts you can target when
running an Ansible script.

See `ssh.cfg` for the SSH configuration that Ansible uses when running.
