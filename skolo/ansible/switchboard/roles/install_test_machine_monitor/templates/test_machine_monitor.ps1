# Launches the test_machine_monitor executable, but not before checking if there
# is an updated executable. Also look for a new version of Foundry Bot, and
# install it if present.
#
# Background: On Windows, you can't delete or overwrite an executable that is
# running, so we always write new executables to (for example)
# test_machine_monitor2.exe. Then this script, which runs only when
# test_machine_monitor.exe is not running, can overwrite
# test_machine_monitor.exe with test_machine_monitor2.exe.

function Update-Executables {
    param (
        [Parameter(Mandatory)]
        [string]$Old,
        [Parameter(Mandatory)]
        [string]$New
    )

    # If the new file exists, overwrite the old file with it.
    if (Test-Path -Path $New -PathType Leaf) {
        # Remove the old one if it exists.
        if (Test-Path -Path $Old -PathType Leaf) {
            Remove-Item -Path $Old -Force -ErrorAction Stop
            Write-Host "The file [$Old] has been deleted."
        }

        # Move the new file into its place.
        Move-Item -Path $New -Destination $Old
        Write-Host "[$New] has been moved into place."
    }
    else {
        # If the file does not exist, then run the existing file.
        Write-Host "Using existing [$Old]; no newer version found."
    }
}

Update-Executables -New '.\test_machine_monitor2.exe' -Old '.\test_machine_monitor.exe'
Update-Executables -New '.\bot.new.exe' -Old '.\bot.1.exe'

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
  --metadata_url={{ metadata_url }} `
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
