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

# Launch test_machine_monitor.
.\test_machine_monitor.exe `
  --config=prod.json `
  --prom_port=:{{ all.prometheus.monitoring.ports.test_machine_monitor}} `
  --username=chrome-bot