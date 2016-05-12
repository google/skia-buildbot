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
try {

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
  addToPath $depotToolsPath
  cmd /c "gclient < nul"
}

banner "Install Python SetupTools."
$fileName = "$tmp\ez_setup.py"
if (!(Test-Path ($fileName))) {
  $url = "http://peak.telecommunity.com/dist/ez_setup.py"
  $webclient.DownloadFile($url, $fileName)
  cmd /c "python $fileName"
    addToPath "$depotToolsPath\python276_bin\Scripts"
}

banner "Install zope.interface."
cmd /c "easy_install zope.interface"

banner "Copy .boto file"
$shell.NameSpace($userDir).copyhere("c:\.boto", 0x14)

banner "Copy _netrc file"
$shell.NameSpace($userDir).copyhere("c:\_netrc", 0x14)
$shell.NameSpace($depotToolsPath).copyhere("c:\_netrc", 0x14)

banner "Copy .gitconfig file"
$shell.NameSpace($depotToolsPath).copyhere("c:\.gitconfig", 0x14)

banner "Copy .bot_password file"
$shell.NameSpace($userDir).copyhere("c:\.bot_password", 0x14)

banner "Start Swarming."
$swarm_slave_dir = "c:\b\swarm_slave"
if (!(Test-Path ($swarm_slave_dir))) {
  cmd /c "python -c `"import urllib; exec urllib.urlopen('https://chromium-swarm.appspot.com/bootstrap').read()`""
}

banner "The Task ended"

} catch {
  log "ERROR:"
  log $_.Message
}

