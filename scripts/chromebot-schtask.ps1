# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Scheduled task for chrome-bot for Windows bots.

$DebugPreference = "Continue"
$ErrorActionPreference = "Stop"
$WarningPreference = "Continue"

$user = "chrome-bot"
$userDir = "c:\Users\$user"
$logFile = "$userDir\schtask.log"

Set-Location -Path $userDir

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

Function unzip($fileName, $folder = "C:\") {
  $zip = $shell.NameSpace($fileName)
  log "Unzip $filename to $folder"
  foreach($item in $zip.items()) {
    log "  extract $item"
    $shell.Namespace($folder).copyhere($item)
  }
}

Function addToPath($dir) {
  # Don't add empty strings.
  If (!$dir) { Return }

  # Add dir to the path.
  $ENV:PATH = $ENV:PATH + ";" + $dir
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

# Create temp directory.
$tmp = "$userDir\tmp"
if (!(Test-Path ($tmp))) {
  new-item $tmp -itemtype directory
}

# Create helpers.
$webclient = New-Object System.Net.WebClient
$shell = new-object -com shell.application

banner "Waiting for network..."
do {
  $ping = test-connection -comp "storage.googleapis.com" -count 1 -Quiet
} until ($ping)

banner "Install depot tools."
$fileName = "$tmp\depot_tools.zip"
$depotToolsPath = "$userDir\depot_tools"
if (!(Test-Path ($depotToolsPath))) {
  $url = "https://storage.googleapis.com/chrome-infra/depot_tools.zip"
  $webclient.DownloadFile($url, $fileName)
  new-item $depotToolsPath -itemtype directory
  unzip $fileName $depotToolsPath
}
addToPath $depotToolsPath
Try {
  $gclient_output = (cmd /c "gclient.bat") 2>&1 | Out-String
  log $gclient_output
} Catch {
  log "gclient failed:"
  log $_.Exception.Message
}
Try {
  cmd /c "update_depot_tools.bat"
} Catch {
  log "update_depot_tools.bat failed:"
  log $_.Exception.Message
}

banner "Manual depot_tools update"
Set-Location -Path $depotToolsPath
$git = (cmd /c "where git") | Out-String
log "git: $git"
log "git fetch"
cmd /c "git.bat fetch"
log "git reset"
cmd /c "git.bat reset --hard origin/master"
$gitstatus = (cmd /c "git.bat status") | Out-String
log $gitstatus
Set-Location -Path $userDir

banner "Ensure Python installed"
$pythonPath = "C:\Python27"
if (!(Test-Path ($pythonPath))) {
  log "Install Python"
  $gsurl = "gs://skia-buildbots/skolo/win/win_package_src/python-2.7.14.amd64.msi"
  $fileName = "$tmp\python.msi"
  & "gsutil" cp $gsurl $fileName
  $pylogFile = "python-install.log"
  $MSIArguments = @(
    "/i"
    $fileName
    "/qn"
    "/norestart"
    "/L*v"
    $pylogFile
  )
  Start-Process "msiexec.exe" -ArgumentList $MSIArguments -Wait -NoNewWindow
  $pylog = Get-Content $pylogFile | Out-String
  log $pylog
}
banner "Ensure Python in Path"
if (!($env:Path -like "*$pythonPath*")) {
  log "Add $pythonPath to Path"
  [Environment]::SetEnvironmentVariable("Path",$pythonPath + ";" + $env:Path,"Machine")
}
banner "Ensure pywin32 installed"
if (!(Test-Path "C:\Windows\System32\pywintypes27.dll")) {
  log "Install pywin32"
  $zipfileName = "$tmp\pywin32.zip"
  $gsurl = "gs://skia-buildbots/skolo/win/win_package_src/pywin32-221.win-amd64-py2.7.zip"
  log "download $gsurl to $zipfileName"
  & "gsutil" cp $gsurl $zipfileName
  $pywin32dest = "$pythonPath\Lib\site-packages\"
  log "unzip $zipfileName to $pywin32dest"
  unzip $zipfileName $pywin32dest
  $gsurl = "gs://skia-buildbots/skolo/win/win_package_src/pywin32-221.win-amd64-py2.7_postinstall.py"
  $pyfileName = "$tmp\pywin32_postinstall.py"
  log "download $gsurl to $pyfileName"
  & "gsutil" cp $gsurl $pyfileName
  log "CD to $pywin32dest"
  Set-Location -Path $pywin32dest
  log "Running $pyfileName from $pywin32dest"
  $pywin32logFile = "python-install.log"
  & "python" -u $pyfileName -silent -install 2>&1 > $pywin32logFile
  $pywin32log = Get-Content $pywin32logFile | Out-String
  log $pywin32log
  Set-Location -Path $userDir
}

banner "Copy .boto file"
$shell.NameSpace($userDir).copyhere("c:\.boto", 0x14)

banner "Copy _netrc file"
$shell.NameSpace($userDir).copyhere("c:\_netrc", 0x14)
$shell.NameSpace($depotToolsPath).copyhere("c:\_netrc", 0x14)

$hostname =(cmd /c "hostname") | Out-String
if ($hostname.StartsWith("ct-")) {
  banner "Installing psutil using pip. Required for CT win bots (skbug/9720)."
  cmd /c "C:\Python27\Scripts\pip.exe install -U psutil"
}
if ($hostname.StartsWith("ct-windows-builder")) {
  banner "Check out Chromium repository"

  cmd /c "git config --global user.name chrome-bot"
  cmd /c "git config --global user.email chrome-bot@chromium.org"
  cmd /c "git config --global core.autocrlf false"
  cmd /c "git config --global core.filemode false"
  cmd /c "git config --global branch.autosetuprebase always"

  $chromiumCheckout = "c:\b\storage\chromium"
  if (!(Test-Path ($chromiumCheckout))) {
    new-item $chromiumCheckout -itemtype directory
    Set-Location -Path $chromiumCheckout
    Try {
      cmd /c "fetch.bat chromium"
    } Catch {
      log "fetch.bat chromium failed:"
      log $_.Exception.Message
    }

    $chromiumSrcDir = "$chromiumCheckout\src"
    Set-Location -Path $chromiumSrcDir
    log "git checkout master"
    cmd /c "git.bat checkout master"
    log "gclient sync"
    cmd /c "gclient.bat sync"

    Set-Location -Path $userDir
  }
}

banner "Copy .gitconfig file"
$shell.NameSpace($depotToolsPath).copyhere("c:\.gitconfig", 0x14)

banner "Create Startup Dir"
$startup_dir = "$userDir\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Startup"
if (!(Test-Path ($startup_dir))) {
  New-Item -ItemType directory -Path $startup_dir
}

banner "Start Swarming."
$swarm_worker_dir = "c:\b\s"
if (!(Test-Path ($swarm_worker_dir))) {
  new-item $swarm_worker_dir -itemtype directory
  $swarming = "https://chromium-swarm.appspot.com"
  if ($hostname.StartsWith("skia-i-") -Or $hostname.StartsWith("ct-")) {
    $swarming = "https://chrome-swarming.appspot.com"
  }
  if ($hostname.StartsWith("skia-d-")) {
    $swarming = "https://chromium-swarm-dev.appspot.com"
  }
  $metadataJson = Invoke-WebRequest -Uri http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token -Headers @{"Metadata-Flavor"="Google"} -UseBasicParsing | ConvertFrom-Json
  curl $swarming/bot_code?bot_id=$hostname -Headers @{"Authorization"="Bearer " + $metadataJson.access_token} -OutFile $swarm_worker_dir/swarming_bot.zip
}
cmd /c "python $swarm_worker_dir/swarming_bot.zip start_bot"

banner "The Task ended"

}
catch
{

log "Caught an exception: $($_.Exception.GetType().FullName)"
log "$($_.Exception.Message)"

}
