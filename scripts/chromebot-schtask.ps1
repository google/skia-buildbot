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

# TODO(borenet): This top-level try/catch is really stupid, but I don't know
# how to get errors logged to a file.
Try {

# Create temp directory.
$tmp = "$userDir\tmp"
if (!(Test-Path ($tmp))) {
  new-item $tmp -itemtype directory
}

# Create helpers.
$webclient = New-Object System.Net.WebClient
$shell = new-object -com shell.application

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
$ENV:DEPOT_TOOLS_UPDATE = "0"
Try {
  $gclient_output = (cmd /c "gclient.bat") 2>&1 | Out-String
  log $gclient_output
} Catch {
  log "gclient failed:"
  log $_.Exception.Message
}

banner "Manual depot_tools update"
Set-Location -Path $depotToolsPath
log "git fetch"
cmd /c "git.bat fetch"
log "git reset"
cmd /c "git.bat reset --hard origin/master"
$gitstatus = (cmd /c "git.bat status") | Out-String
log $gitstatus
Set-Location -Path $userDir

banner "Copy .boto file"
$shell.NameSpace($userDir).copyhere("c:\.boto", 0x14)

banner "Copy _netrc file"
$shell.NameSpace($userDir).copyhere("c:\_netrc", 0x14)
$shell.NameSpace($depotToolsPath).copyhere("c:\_netrc", 0x14)

banner "Copy .gitconfig file"
$shell.NameSpace($depotToolsPath).copyhere("c:\.gitconfig", 0x14)

banner "Copy .bot_password file"
$shell.NameSpace($userDir).copyhere("c:\.bot_password", 0x14)

banner "Create Startup Dir"
$startup_dir = "$userDir\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Startup"
if (!(Test-Path ($startup_dir))) {
  New-Item -ItemType directory -Path $startup_dir
}

banner "Start Swarming."
$swarm_slave_dir = "c:\b\swarm_slave"
if (!(Test-Path ($swarm_slave_dir))) {
  cmd /c "python -c `"import urllib; exec urllib.urlopen('https://chromium-swarm.appspot.com/bootstrap').read()`""
}

banner "The Task ended"

} Catch {
  log "ERROR:"
  log $_.Exception.Message
  log $error[0]
}

