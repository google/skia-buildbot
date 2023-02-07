$DebugPreference = "Continue"
$ErrorActionPreference = "Stop"
$WarningPreference = "Continue"

$username = "chrome-bot"
$password = "CHROME_BOT_PASSWORD"
$domain = $env:userdomain
$logFile = "C:\gce_setup.log"

Function log($msg) {
  Write-Debug $msg
  Add-Content $logFile "$msg`n"
  try {
    # Write to GCE serial port output (console), if available.
    $port= new-Object System.IO.Ports.SerialPort COM1,9600,None,8,one
    $port.open()
    $port.WriteLine($msg)
    $port.close()
  } catch {}
}

Function banner($title) {
  $bannerWidth = 80
  $padChar = "*"
  $titleLine = " $title "
  $len = $titleLine.length
  $padding = 0
  $extra = $bannerWidth - $len
  if ($extra -ge 4) {
    $padding = $extra / 2
  }
  $titleLine = $titleLine.PadLeft($padding + $len, $padChar)
  $titleLine = $titleLine.PadRight($bannerWidth, $padChar)
  log ""
  log "".PadRight($bannerWidth, $padChar)
  log $titleLine
  log "".PadRight($bannerWidth, $padChar)
  log ""
}

log "setup-win-ansible.ps1: Begin."

try {
  # Create chrome-bot user.
  banner "Create user $username"
  $sspassword = ConvertTo-SecureString $password -AsPlainText -Force
  New-LocalUser -Name $username -Password $sspassword -PasswordNeverExpires -UserMayNotChangePassword -AccountNeverExpires
  Add-LocalGroupMember -Group "Administrators" -Member "$username"

  # Enable SSH.
  banner "Install SSH and set it to start on startup"
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType 'Automatic'
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -Value "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe" -PropertyType String -Force
} catch {
  log "Caught an exception: $($_.Exception.GetType().FullName)"
  log "$($_.Exception.Message)"
}

log "setup-win-ansible.ps1: End."
