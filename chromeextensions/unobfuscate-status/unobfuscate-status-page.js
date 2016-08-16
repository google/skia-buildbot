// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function getMappingAndReplace() {
  sk.get('https://internal.skia.org/mapping/').then(function(resp) {
    replace(JSON.parse(resp));
  }).catch(function() {
    console.error('Error connecting to internal.skia.org');
  });

  setTimeout(getMappingAndReplace, 10000);  // 10 seconds
}

function handleClick(code, real, className) {
  var checkExist = setInterval(function() {
    var popupName = className.includes('builder') ? 'builder-popup-sk' : 'build-popup-sk';
    var parentPopup = document.getElementsByTagName(popupName)[0]
    if (parentPopup && parentPopup.style.display == '') {
      var popupElems = document.getElementsByClassName(popupName)
      for (var j = 0; j < popupElems.length; j++) {
        var popupElem = popupElems[j]
        if (popupElem.text && popupElem.text.includes(code)) {
          popupElem.text = popupElem.text.replace(new RegExp(code, 'gi'), real);
        }
      }
      clearInterval(checkExist);
    }
  }, 100); // check every 100ms
}

function replace(codeNamesToReal) {
  var builders = document.getElementsByClassName('builders');
  if (builders.length > 0) {
    var tags = ['builder-title', 'build_top', 'build_middle', 'build_bottom', 'build_single'];
    var allElems = []
    for (var i = 0; i < tags.length; i++) {
      var elems = document.getElementsByClassName(tags[i]);
      if (elems) {
        var elemsArr = Array.prototype.slice.call(elems);
        allElems = allElems.concat(elemsArr);
      }
    }
    for (var i = 0; i < allElems.length; i++) {
      var elem = allElems[i];
      codeNamesToReal.forEach(function(codeNameToReal) {
        var code = codeNameToReal['Codename'];
        var real = codeNameToReal['Target'];
        var new_title = elem.title.replace(new RegExp(code, 'gi'), real);
        if (elem.title != new_title) {
          elem.title = new_title

          // Set event listener to update text in popup.
          elem.addEventListener('click', handleClick.bind(this, code, real, elem.className));
        }
      });
    }
  }

  setTimeout(function() {
    replace(codeNamesToReal);
  }, 1000);  // 1 second
}

// Replace all code names with real names.
getMappingAndReplace();

// Change CSS of the status page for some keyboard combinations.
document.addEventListener('keyup', function(e) {
  var green = 'rgba(102, 166, 30, 0.298039)'
  var red = 'rgb(217, 95, 2)'
  var purple = 'rgb(117, 112, 179)'

  // Alt+Shift+G
  if (e.altKey && e.shiftKey && e.keyCode == 71) {
    $$('div').forEach(function(d) {
      var color = d.style.backgroundColor;
      if (color == red || color == purple) {
        d.style.backgroundColor = green;
      }
    });
  }
  // Alt+Shift+F
  if (e.altKey && e.shiftKey && e.keyCode == 70) {
    $$('div').forEach(function(d) {
      var color = d.style.backgroundColor;
      if (color == green || color == purple) {
        d.style.backgroundColor = red;
      }
    });
  }
  // Alt+Shift+P
  if (e.altKey && e.shiftKey && e.keyCode == 80) {
    $$('div').forEach(function(d) {
      var color = d.style.backgroundColor;
      if (color == green || color == red) {
        d.style.backgroundColor = purple;
      }
    });
  }
});
