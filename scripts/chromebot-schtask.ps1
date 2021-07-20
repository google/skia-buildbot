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
# If this path changes, be sure to update win_startup.ps1 to match.
$pythonPath = "C:\Python38"
if (!(Test-Path ($pythonPath))) {
  log "Install Python"
  $gsurl = "gs://skia-buildbots/skolo/win/win_package_src/python-3.8.10-amd64.exe"
  $fileName = "$tmp\python-install.exe"
  & "gsutil" cp $gsurl $fileName
  $pylog = & $fileName /quiet Include_test=0 Include_pip=1 Include_doc=0 TargetDir=$pythonPath | Out-String
  log $pylog
}

banner "Ensure pywin32 installed"
$pipList = & $pythonPath\Scripts\pip list | Out-String
if (!($pipList -like '*pywin32*')) {
  log "Install pywin32"
  $pipLog = & $pythonPath\Scripts\pip install pywin32 | Out-String
  log $pipLog
  $pyfileName = "$pythonPath\Scripts\pywin32_postinstall.py"
  log "Running $pyfileName"
  $pywin32log = & "python" -u $pyfileName -silent -install | Out-String
  log $pywin32log
}

banner "Copy .boto file"
$shell.NameSpace($userDir).copyhere("c:\.boto", 0x14)

banner "Copy _netrc file"
$shell.NameSpace($userDir).copyhere("c:\_netrc", 0x14)
$shell.NameSpace($depotToolsPath).copyhere("c:\_netrc", 0x14)

$hostname =(cmd /c "hostname") | Out-String
if ($hostname.StartsWith("ct-")) {
  banner "Installing psutil using pip. Required for CT win bots (skbug/9720)."
  cmd /c "$pythonPath\Scripts\pip.exe install -U psutil"
  banner "Installing requests using pip. Required for CT win bots (skbug/10960)."
  cmd /c "$pythonPath\Scripts\pip.exe install -U requests"
  banner "Installing six using pip. Required for CT win bots (skbug/11888)."
  cmd /c "$pythonPath\Scripts\pip.exe install -U six"
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
