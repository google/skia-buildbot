// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

(function() {
  function toggle(e) {
    e.target.nextElementSibling.classList.toggle("display");
  }

  // hookExpando finds all the expander buttons and adds a handler
  // that toggles the 'display' class on its next sibling element.
  // TODO(jcgregorio) Switch to details/summary once we have a polyfill in place.
  function hookExpando() {
    [].forEach.call(document.querySelectorAll(".expander"), function(ele) {
      this.addEventListener('click', toggle);
    });
  };
  document.addEventListener('DOMContentLoaded', hookExpando);
})();
