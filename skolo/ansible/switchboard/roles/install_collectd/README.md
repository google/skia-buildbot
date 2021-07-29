# Role Name

`install_collectd`

## Description

Installs `collectd` along with a configuration file that sends the data to
Prometheus.

## Example Playbook

    - hosts: '{{ variable_hosts }}'

      roles:
        - install_collectd
