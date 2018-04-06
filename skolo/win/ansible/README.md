Windows Skolo Ansible Scripts
-----------------------------

See "Windows new bot setup" doc for context.

If Ansible is not installed, run `sudo pip install ansible`. Playbooks that
require a minimum version should include a comment indicating so. To upgrade,
run `sudo pip install ansible --upgrade`.

There are two Ansible inventory files in this dir, win-02-hosts and
win-03-hosts. There is also a group_vars dir and a win_package_src dir
that I can't check in. The group_vars include these variable settings:

```yaml
ansible_user: chrome-bot
ansible_password: <redacted>
ansible_port: 5986
ansible_connection: winrm
ansible_winrm_transport: credssp
ansible_winrm_server_cert_validation: ignore
win_package_src: /home/chrome-bot/ansible/win_package_src/
```

Contents of win_package_src are on GCS at
gs://skia-buildbots/skolo/win/win_package_src

Example command: `ansible-playbook -i win-02-hosts setup-skolo-bot.yml`
