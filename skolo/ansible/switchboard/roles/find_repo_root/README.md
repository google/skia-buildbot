# Role Name

`find_repo_root`

## Role Variables

Finds the root of the `skiabot` repo and exports it as a variable.

## Example Playbook

    - hosts: servers
      roles:
        - find_repo_root
      tasks:
        - name: echo repo_root
          debug:
            msg: "The root of this repo is: {{ repo_root }}"
