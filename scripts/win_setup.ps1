# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Setup script for Windows bots.

$DebugPreference = "Continue"
$ErrorActionPreference = "Stop"
$WarningPreference = "Continue"

$user = "chrome-bot"
$userDir = "c:\Users\$user"
$logFile = "$userDir\win_setup.log"

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

  # Retrieve the current PATH from the registry.
  $envRegPath = ('Registry::HKEY_LOCAL_MACHINE\System\CurrentControlSet\' +
                 'Control\Session Manager\Environment')
  $oldPath = (Get-ItemProperty -Path $envRegPath -Name PATH).Path

  # Don't add duplicates to PATH.
  $oldPath.Split(";") | ForEach { If ($_ -eq $dir) { Return } }

  # Override PATH
  $newPath=$oldPath+’;’+$dir
  Set-ItemProperty -Path $envRegPath -Name PATH –Value $newPath
  $actualPath = (Get-ItemProperty -Path $envRegPath -Name PATH).Path
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

# Update DNS Server for internet access.
$wmi = `
    Get-WmiObject win32_networkadapterconfiguration -filter "ipenabled = 'true'"
$wmi.SetDNSServerSearchOrder("8.8.8.8")

# Create temp directory.
$tmp = "$userDir\tmp"
if (!(Test-Path ($tmp))) {
  new-item $tmp -itemtype directory
}

# Create helpers.
$webclient = New-Object System.Net.WebClient
$shell = new-object -com shell.application

banner "Install Visual Studio C++ 2008 redistributable (x86)."
$fileName = "$tmp\vcredist_x86.exe"
if (!(Test-Path ($fileName))) {
  $url = ("http://download.microsoft.com/download/1/1/1/" +
          "1116b75a-9ec3-481a-a3c8-1777b5381140/vcredist_x86.exe")
  $webclient.DownloadFile($url, $fileName)
  cmd /c $fileName /q
}

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
cmd /c "gclient sync --force --verbose"

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
