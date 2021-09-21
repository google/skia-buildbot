# get_ansible_binaries

Downloads crossplatform builds of applications that need to be distributed into
the skolo. See http://go/skia-ansible-binaries.

## Requirements

The binaries should already be uploaded.

## Role Arguments

### get_ansible_binaries_application

The name of the application, i.e. APP from the design doc, such as:
test_machine_monitor.

### get_ansible_binaries_version

The version to deploy, optional variable to override the default which is to
push the version as recorded in the k8s-config repo.

## Role variables

The role will create a variable of the name `get_ansible_binaries_directory`
that has a path attribute with the location of the downloaded binaries.

## Example Playbook

This role is designed to be used in other roles.

        - name: Load test_machine_monitor executables.
          import_role:
            name: get_ansible_binaries
          vars:
            get_ansible_binaries_application: test_machine_monitor
            get_ansible_binaries_version: '{{ test_machine_monitor_version }}'

        - name: Copy over executable.
          become: yes
          copy:
            src:
              "{{ get_ansible_binaries_directory.path }}/build/{{ ansible_facts['system']
              }}/{{ ansible_facts['architecture'] }}/test_machine_monitor"
            dest: /usr/local/bin/test_machine_monitor
            owner: root
            group: root
            mode: 0755
