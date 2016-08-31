// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// A Series of time based utilites for Milo.


(function(window) {
  'use strict';

  var milo = window.milo || {};

  /**
   * Given a Date, return a time string in the user's local timezone.
   */
  milo.formatDate = function(t) {
    if (!t || t.toString() == "Invalid Date") {
        return null;
    }
    var shortDayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    var offset = -(new Date()).getTimezoneOffset();
    var offsetHr = Math.abs(Math.round(offset / 60));
    var offsetMin = Math.abs(Math.abs(offset) - (offsetHr * 60));
    if (offsetHr < 10) {
      offsetHr = '0' + offsetHr;
    }
    if (offsetMin < 10) {
      offsetMin = '0' + offsetMin;
    }

    var month = (t.getMonth() + 1);
    if (month < 10) {
      month = '0' + month;
    }
    var date = t.getDate();
    if (date < 10) {
      date = '0' + date;
    }
    var s = shortDayNames[t.getDay()] + ', ';
    s += t.getFullYear() + '-' + month + '-' + date + ' ';
    s += t.toLocaleTimeString();

    return s;
  };

  milo.makeTimesLocal = function() {
    var timeSpans = document.getElementsByClassName('local-time');
    for (var i = 0; i < timeSpans.length; i++) {
      var span = timeSpans[i];
      try {
        var oldTimestamp = span.innerText;
        var timestamp = span.getAttribute('data-timestamp');
        var date = new Date(parseInt(timestamp, 10));
        var newTimestamp = milo.formatDate(date);
        if (newTimestamp != null) {
          span.innerText = newTimestamp;
          span.setAttribute("title", oldTimestamp)
        }
      }
      catch (e) {
        console.error('could not convert time of span', span, 'to local:', e)
      }
    }
  };

  window.milo = milo;

}(window));
