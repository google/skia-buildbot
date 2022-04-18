# Role Name

`install_compui`

## Description

Installs `comp-ui-cron-job` to the machine and arranges for it to run daily.

## Variables Required

Requires `gather_facts` to detect the target operating system.

## Example Playbook

```
# Install test_machine_monitor that doesn't launch Swarming.
- hosts: compui
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_compui

```

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the cipd package via the
//comp-ui Makefile:

```
$ cd comp-ui
$ make release_compui
```

Then visit http://go/cipd/p/skia/internal/comp-ui-cron-job/+/ (or look in the
logs) to find the version for that build and pass is to this playbook via
--extra-vars.

For example:

```
$ ansible-playbook ./switchboard/install_compui.yml \
  --extra-vars comp_ui_cron_job_version=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```
