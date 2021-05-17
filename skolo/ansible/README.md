# Skolo Ansible Access

These Ansible scripts can be run from a desktop machine and not from a lab
computer.

Visit http://go/skolo-maintenance#heading=h.or4jzu6r2mzn for instructions on how
to set up to run these commands.

## Tips

Runs might fail for a small number of hosts, you can re-run a script for a
specific host by passing `-l (hostname)` to the `ansible-playbook` command.
