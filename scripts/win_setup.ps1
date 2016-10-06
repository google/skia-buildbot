
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
  $webclient.DownloadFile($url, $fileName)
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

banner "Create .bot_password"
$bot_password = @"
INSERTFILE(/tmp/.bot_password)
"@
Set-Content C:\.bot_password $bot_password

banner "Create .gitconfig"
$gitconfig_contents = @"
INSERTFILE(/tmp/.gitconfig)
"@
Set-Content C:\.gitconfig $gitconfig_contents

banner "Update hosts file."
$additional_hosts = @"
`n
INSERTFILE(/tmp/chrome_master_host)
"@
Add-Content c:\Windows\System32\drivers\etc\hosts $additional_hosts

banner "Download chrome-bot's scheduled task powershell script"
$metadataclient = New-Object System.Net.WebClient
$metadataclient.Headers.Add("Metadata-Flavor", "Google")
$url = "http://metadata/computeMetadata/v1/instance/attributes/chromebot-schtask-ps1"
$chromebotSchTask = "c:\chromebot-schtask.ps1"
$data = $metadataclient.DownloadString($url)
log $data
Set-Content $chromebotSchTask $data

banner "Set chrome-bot's scheduled task"
schtasks /Create /TN skiabot /SC ONSTART /TR "powershell.exe -executionpolicy Unrestricted -file $chromebotSchTask" /RU $username /RP $password /F /RL LIMITED

$bot_dir = "C:\b"
banner "Create $bot_dir"
New-Item -ItemType directory -Path $bot_dir
$acl = Get-Acl $bot_dir
$acl.SetOwner([System.Security.Principal.NTAccount] $username)
Set-Acl $bot_dir $acl

banner "The startup script completed"
