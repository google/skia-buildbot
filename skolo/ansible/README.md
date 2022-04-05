# Skolo Ansible Access

These Ansible scripts can be run from a desktop machine and not from a lab
computer.

Visit http://go/skolo-maintenance#heading=h.or4jzu6r2mzn for instructions on how
to set up to run these commands.

## Tips

Runs might fail for a small number of hosts, you can re-run a script for a
specific host by passing `--limit (hostname)` to the `ansible-playbook` command.

## Notes

See `hosts.yml` for all the hosts and groups of hosts you can target when
running an Ansible script, as well as for the definitions of broadly scoped
variables.

See `ssh.cfg` for the SSH configuration that Ansible uses when running.

## Adding new machines
 1. Add entries to `./hosts.yml`.
    1. If you are adding to an existing group of machines (e.g. adding an extra mac to rack 1),
       then you just need to had the host name with the others.
    2. If you are creating a new group of machines (e.g. a new OS or purpose), then you will to set
       some variables for those new machines (e.g. `eskia_test_machines`) in addition to specifying
       the hostnames of the machines in that group (e.g. `rack5_linux_eskia`).
 2. Add new entries to `./ssh.cfg` if necessary (e.g. new rack or naming convention)
 3. Check that you can ssh into those machines, e.g. `ssh skia-i-eskia01`.
 4. To save typing passwords on future steps, copy the authorized SSH keys to the new machines.
    1. Update the hosts section of `./switchboard/update-authorized-keys.yml` to contain the new
       group of machines, if necessary.
    2. Run that playbook targeting your machines or group, e.g.
       `ansible-playbook ./switchboard/update-authorized-keys.yml --limit rack5_linux_eskia` or
       `ansible-playbook ./switchboard/update-authorized-keys.yml --limit skia-e-win-355`
 5. If you are adding a new group of machines, create a `./switchboard/foo.yml` to describe the
    [roles](https://docs.ansible.com/ansible/2.9/user_guide/playbooks_reuse_roles.html), (similar
    to functions) that you want to run for your machines.
 6. Run the appropriate setup script for your new machines. Some roles require sudo access,
    so be sure to include the `--ask-become-pass` argument. e.g.
   `ansible-playbook ./switchboard/eskia.yml --limit skia-i-eskia01 --ask-become-pass`
 7. SSH into your machines to verify things are set up correctly (e.g. swarming is running).
