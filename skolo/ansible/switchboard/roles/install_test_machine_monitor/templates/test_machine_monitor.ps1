# Launches the test_machine_monitor executable, but not before checking if there
# is an updated executable.
#
# Background: On Windows you can't delete or overwrite an executable that is
# running, so we always write new executables to test_machine_monitor2.exe and
# then this script, which only runs when test_machine_monitor.exe is not
# running, can then overwrite test_machine_monitor.exe with
# test_machine_monitor2.exe.

$newfile = '.\test_machine_monitor2.exe'
$oldfile = '.\test_machine_monitor.exe'

# If the file exists, move it over test_machine_monitor.exe.
if (Test-Path -Path $newfile -PathType Leaf) {
    # Remove the old one if it exists.
    if (Test-Path -Path $oldfile -PathType Leaf) {
        Remove-Item -Path $oldfile -Force -ErrorAction Stop
        Write-Host "The file [$oldfile] has been deleted."
    }

    # Overwrite the existing test_machine_monitor.exe.
    Move-Item -Path $newfile -Destination $oldfile
    Write-Host "[$newfile] has been overwritten."
}
else {
    # If the file does not exist, then run the existing file.
    Write-Host "Running existing [$oldfile], no newer version found."
}

# Set environment.
{% if install_test_machine_monitor__start_swarming is true %}
$Env:SWARMING_BOT_ID = '{{ ansible_facts['hostname'] }}'
$Env:SWARMING_EXTERNAL_BOT_SETUP = 1
{% endif %}

# Give network a chance to become ready. test_machine_monitor.exe fails to launch if the network is
# still offline. The sleep interval was chosen arbitrarily.
sleep 10

# Launch test_machine_monitor.
.\test_machine_monitor.exe `
  --config=prod.json `
  --prom_port=:{{ all.prometheus.monitoring.ports.test_machine_monitor}} `
  --metadata_url=http://metadata:{{ all.metadata_server_port }}/computeMetadata/v1/instance/service-accounts/default/token `
  {% if install_test_machine_monitor__start_swarming is true %}
  --python_exe={{ win_python3_path }}\python.exe `
  --start_swarming `
  --swarming_bot_zip=C:\b\s\swarming_bot.zip `
  {% endif %}
  {% if install_test_machine_monitor__start_foundry_bot is true %}
  --start_foundry_bot `
  --foundry_bot_path=C:\Users\{{ skolo_account }}\bin\bot.1.exe `
  {% endif %}
  --username=chrome-bot
