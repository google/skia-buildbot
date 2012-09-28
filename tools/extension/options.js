// Copyright (c) 2012 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @fileoverview Code related to the options page. Allows users of the extension
 * to save settings such as the update interval and which builders to watch.
 */
var bg = chrome.extension.getBackgroundPage();
var nextPatternIndex;

/**
 * Toggles whether the Save button is enabled or disabled.
 */
function onChangeCallback() {
  document.getElementById('save').disabled = false;
}

/**
 * Initializes the options page and pulls in any saved settings.
 */
function init() {
  var input = document.getElementById('interval');
  input.value = bg.localStorage['check_interval_sec'];
  input.onchange = onChangeCallback;
  input.onkeyup = onChangeCallback;
  var patterns = JSON.parse(bg.localStorage['core_builder_patterns']);
  var patternParent = document.getElementById('patternParent');
  for (var i = 0; i < patterns.length; i++) {
    input = document.createElement('input');
    input.name = 'pattern' + i;
    input.id = 'pattern' + i;
    input.value = patterns[i];
    input.onchange = onChangeCallback;
    input.onkeyup = onChangeCallback;
    if (i == 0)
      input.required = true;
    var container = document.createElement('div');
    container.appendChild(input);
    patternParent.appendChild(container);
  }
  nextPatternIndex = patterns.length;

  document.getElementById('add').onclick = addPatternField;
  document.getElementById('reset').onclick = resetToDefault;
  document.getElementById('save').onclick = saveOptions;
  document.getElementById('save').disabled = true;
}

/**
 * Updates the message element in the options page.
 * @param {string} str The string to set the message to.
 */
function message(str) {
  document.getElementById('message').innerHTML = str;
  if (str)
    setTimeout(function () { message(''); }, 5 * 1000);
}

/**
 * Resets the options back to the default.
 * @return {boolean} Always returns false;
 */
function resetToDefault() {
  document.getElementById('patternParent').innerHTML = '';
  bg.initOptions();
  init();
  return false;
}

/**
 * Saves the options that were entered by the user.
 * @return {boolean} Always returns false.
 */
function saveOptions() {
  document.getElementById('save').disabled = true;
  var interval = document.getElementById('interval');
  if (!interval.validity.valid) {
    interval.focus();
    return false;
  }
  bg.localStorage['check_interval_sec'] = interval.value;

  var patterns = [];
  for (var i = 0; i < nextPatternIndex; i++) {
    var input = document.getElementById('pattern' + i);
    if (!input)
      continue;
    if (input.value)
      patterns.push(input.value);
  }
  if (patterns.length <= 0) {
    document.getElementById('pattern0').focus();
    return false;
  }
  bg.localStorage['core_builder_patterns'] = JSON.stringify(patterns);
  bg.loadOptions();
  bg.updateBadge();
  message('Options have been saved.');
  return false;
}

/**
 * Adds a new input box so the user can add more core builder patterns.
 * @return {boolean} Always returns false.
 */
function addPatternField() {
  var input = document.createElement('input');
  input.name = 'pattern' + nextPatternIndex;
  input.id = 'pattern' + nextPatternIndex;
  input.onchange = onChangeCallback;
  input.onkeyup = onChangeCallback;
  var container = document.createElement('div');
  container.appendChild(input);
  document.getElementById('patternParent').appendChild(container);
  input.focus();
  nextPatternIndex++;
  return false;
}

init();
