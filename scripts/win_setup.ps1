
$DebugPreference = "Continue"
$ErrorActionPreference = "Stop"
$WarningPreference = "Continue"

$comp = hostname
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

try
{

# See https://stackoverflow.com/questions/48603203/powershell-invoke-webrequest-throws-webcmdletresponseexception
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12

# Create helpers.
$webclient = New-Object System.Net.WebClient
$shell = new-object -com shell.application

# Update DNS Server for internet access.
$wmi = `
    Get-WmiObject win32_networkadapterconfiguration -filter "ipenabled = 'true'"
$wmi.SetDNSServerSearchOrder("8.8.8.8")

banner "Install Visual Studio C++ 2008 redistributable (x86)"
$downloadDir = "C:\downloads"
if (!(Test-Path ($downloadDir))) {
  New-Item -path "$downloadDir" -type directory
}
Set-Location -Path $downloadDir
$fileName = "$downloadDir\vcredist_x86.exe"
if (!(Test-Path ($fileName))) {
  $url = ("http://download.microsoft.com/download/1/1/1/" +
          "1116b75a-9ec3-481a-a3c8-1777b5381140/vcredist_x86.exe")
  # Retry 5 times
  for ($i=1; $i -le 5; $i++) {
    try
    {
      $webclient.DownloadFile($url, $fileName)
      break
    }
    catch
    {
      log "Error downloading file from ${url}: $($_.Exception.GetType().FullName)"
      log "$($_.Exception.Message)"
    }
    Start-Sleep -s 10
  }
  cmd /c $fileName /q
}

banner "Install CMake"
Set-Location -Path $downloadDir
$fileName = "$downloadDir\cmake-3.5.1-win32-x86.msi"
if (!(Test-Path ($fileName))) {
  $url = "https://cmake.org/files/v3.5/cmake-3.5.1-win32-x86.msi"
  $webclient.DownloadFile($url, $fileName)
  cmd /c $fileName /q
}

banner "Create _netrc file"
$netrc_contents = @"
INSERTFILE(/tmp/.netrc)
"@
Set-Content C:\_netrc $netrc_contents

banner "Create .gitconfig"
$gitconfig_contents = @"
INSERTFILE(/tmp/.gitconfig)
"@
Set-Content C:\.gitconfig $gitconfig_contents

banner "Create user $username"
# Win2k8 has an older Powershell that doesn't support New-LocalUser.
If (($PSVersionTable.PSVersion.Major -eq 5 -and $PSVersionTable.PSVersion.Minor -ge 1) -or
    $PSVersionTable.PSVersion.Major -gt 5) {
  $sspassword = ConvertTo-SecureString $password -AsPlainText -Force
  New-LocalUser -Name $username -Password $sspassword -PasswordNeverExpires -UserMayNotChangePassword -AccountNeverExpires
  Add-LocalGroupMember -Group "Administrators" -Member "$username"
} Else {
  # /y seems to bypass the warning about passwords longer than 14 characters not working in Win2000.
  net user "$username" "$password" /add /y
  net localgroup "Administrators" "$username" /add
  wmic useraccount where "Name='$username" set PasswordExpires=FALSE
}

banner "Download chrome-bot's scheduled task powershell script"
$metadataclient = New-Object System.Net.WebClient
$metadataclient.Headers.Add("Metadata-Flavor", "Google")
$url = "http://metadata/computeMetadata/v1/instance/attributes/chromebot-schtask-ps1"
$chromebotSchTask = "c:\chromebot-schtask.ps1"
$data = $metadataclient.DownloadString($url)
log $data
Set-Content $chromebotSchTask $data

banner "Set chrome-bot's scheduled task"
schtasks /Create /IT /TN skiabot /SC ONLOGON /TR "powershell.exe -executionpolicy Unrestricted -file $chromebotSchTask" /RU $username /RP $password /F /RL HIGHEST

$bot_dir = "C:\b"
banner "Create $bot_dir"
New-Item -ItemType directory -Path $bot_dir
$acl = Get-Acl $bot_dir
$acl.SetOwner([System.Security.Principal.NTAccount] $username)
Set-Acl $bot_dir $acl

banner "The setup script completed"

}
catch
{

log "Caught an exception: $($_.Exception.GetType().FullName)"
log "$($_.Exception.Message)"

}
