
$DebugPreference = "Continue"
$ErrorActionPreference = "Stop"
$WarningPreference = "Continue"

$comp = hostname
$username = "chrome-bot"
$password = "CHROME_BOT_PASSWORD"
$domain = $env:userdomain
$userDir = "C:\Users\$username"
$logFile = "C:\gce_startup.log"

Function log($msg) {
  Write-Debug $msg
  Add-Content $logFile "$msg`n"
}

Function addToRegistryPath($dir) {
  # Don't add empty strings.
  If (!$dir) { Return }

  # Retrieve the current PATH from the registry.
  $envRegPath = ("Registry::HKEY_LOCAL_MACHINE\System\CurrentControlSet\" +
                 "Control\Session Manager\Environment")
  $oldPath = (Get-ItemProperty -Path $envRegPath -Name PATH).Path

  # Don't add duplicates to PATH.
  $oldPath.Split(";") | ForEach { If ($_ -eq $dir) { Return } }

  # Override PATH
  $newPath=$oldPath+";"+$dir
  Set-ItemProperty -Path $envRegPath -Name PATH -Value $newPath
  $actualPath = (Get-ItemProperty -Path $envRegPath -Name PATH).Path
  log $actualPath
  $ENV:PATH = $actualPath
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

banner "Add to registry PATH"
addToRegistryPath "$userDir\depot_tools"
addToRegistryPath "$userDir\depot_tools\python276_bin\Scripts"

banner "Create .boto file"
$boto_contents = (
    "[Credentials]`n" +
    "GS_ACCESS_KEY_ID`n" +
    "GS_SECRET_ACCESS_KEY`n" +
    "[Boto]`n"
    )
Set-Content C:\.boto $boto_contents

banner "Create _netrc file"
$netrc_contents = @"
INSERTFILE(~/.netrc)
"@
Set-Content C:\_netrc $netrc_contents

banner "Create .bot_password"
$bot_password = @"
INSERTFILE(~/.bot_password)
"@
Set-Content C:\.bot_password $bot_password

banner "Update hosts file."
$additional_hosts = @"
`n
INSERTFILE(~/chrome_master_host)
"@
Add-Content c:\Windows\System32\drivers\etc\hosts $additional_hosts

banner "Download chrome-bot's scheduled task powershell script"
$url = ("https://skia.googlesource.com/buildbot/+/master/scripts/" +
        "chromebot-schtask.ps1?format=TEXT")
$b64SchTask = "C:\b64-chromebot-schtask"
$webclient.DownloadFile($url, $b64SchTask)
$b64Data = Get-Content $b64SchTask
$chromebotSchTask = "C:\chomebot-schtask.ps1"
[System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($b64Data)) | Out-File -Encoding "ASCII" $chromebotSchTask

banner "Set chrome-bot's scheduled task"
schtasks /Create /TN skiabot /SC ONSTART /TR "powershell.exe -executionpolicy Unrestricted -file $chromebotSchTask" /RU $username /RP $password /F /RL HIGHEST

banner "The startup script completed"
