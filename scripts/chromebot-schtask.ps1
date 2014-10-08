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
  $url = "https://src.chromium.org/svn/trunk/tools/depot_tools.zip"
  $webclient.DownloadFile($url, $fileName)
  unzip $fileName $userDir
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

banner "Copy .bot_password file"
$shell.NameSpace($userDir).copyhere("c:\.bot_password", 0x14)

banner "Download Buildbot scripts."
$gclientSpec = ( `
  "`"solutions = [{ " +
  "'name': 'buildbot'," +
  "'url': 'https://skia.googlesource.com/buildbot.git'," +
  "'deps_file': 'DEPS'," +
  "'managed': True," +
  "'custom_deps': {}," +
  "'safesync_url': ''," +
  "},{ " +
  "'name': 'src'," +
  "'url': 'https://chromium.googlesource.com/chromium/src.git'," +
  "'deps_file': '.DEPS.git'," +
  "'managed': True," +
  "'custom_deps': {}," +
  "'safesync_url': ''," +
  "},]`"")
cmd /c "gclient config --spec=$gclientSpec"
cmd /c "gclient sync --force --verbose -j1"

banner "Copy WinDbg Files"
$winDbgFolder = "c:\DbgHelp"
if (!(Test-Path ($winDbgFolder))) {
  new-item $winDbgFolder -itemtype directory
}
$x86lib = ("$depotToolsPath\win_toolchain\vs2013_files\win8sdk\Debuggers\lib\" +
           "x86\dbghelp.lib")
$shell.NameSpace($winDbgFolder).copyhere($x86lib, 0x14)
if (!(Test-Path ("$winDbgFolder\x64"))) {
  new-item "$winDbgFolder\x64" -itemtype directory
}
$x64lib = ("$depotToolsPath\win_toolchain\vs2013_files\win8sdk\Debuggers\lib\" +
           "x64\dbghelp.lib")
$shell.NameSpace("$winDbgFolder\x64").copyhere($x64lib, 0x14)

banner "Launch the Slave"
cd buildbot
cmd /c "call python scripts\launch_slaves.py"

banner "The Task ended"

} catch {
  log "ERROR:"
  log $_.Message
}

