# Role Name

`install_test_machine_monitor`

## Description

Installs `test_machine_monitor` to the machine and arranges for it to run at
startup. Installs Foundry Bot as well, which TMM optionally uses to field Bazel
RBE tasks. Does _not_ restart `test_machine_monitor` or Foundry Bot if an old
copy is already running; reboot to bring up the new version.

## Variables Required

Requires `gather_facts` to detect the target operating system.

## Optional Variables

`install_test_machine_monitor__start_swarming` controls whether
`test_machine_monitor` should be passed the `--start_swarming` flag.

`start_foundry_bot` controls whether `test_machine_monitor` should run Foundry
Bot and field Bazel RBE requests, via the `--start_foundry_bot` flag.

`install_test_machine_monitor__linux_run_under_desktop` determines if the
Swarming launched from `test_machine_monitor` needs access to a graphical
interface such as X Windows.

## Example Playbook

```
# Install test_machine_monitor that doesn't launch Swarming.
- hosts: rpis
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_test_machine_monitor

```

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the cipd package via the
//machine Makefile:

```
$ cd machine
$ make build_and_upload_test_machine_monitor
```

Then visit http://go/cipd/p/skia/internal/test_machine_monitor/+/ (or look
in the logs) to find the version for that build and pass it to this
playbook via --extra-vars.

For example:

```
$ ansible-playbook ./switchboard/install_test_machine_monitor.yml \
  -l skia-rpi2-rack4-shelf4-006 \
  --extra-vars test_machine_monitor_version=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```

A similar `foundry_bot_version` variable specifies the version of Foundry Bot to
be deployed:

```
$ ansible-playbook ./switchboard/install_test_machine_monitor.yml \
  -l skia-rpi2-rack4-shelf4-006 \
  --extra-vars foundry_bot_version=2022-12-30T02_28_24Z-erikrose-8e5ecae-dirty
```
