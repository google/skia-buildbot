/**
 * common.js is a set of common functions used across all of skiaperf.
 *
 * Everything is scoped to 'sk' except $$ and $$$ which are global since they
 * are used so often.
 *
 */

/**
 * $$ returns a real JS array of DOM elements that match the CSS query selector.
 *
 * A shortcut for jQuery-like $ behavior.
 **/
function $$(query, ele) {
  if (!ele) {
    ele = document;
  }
  return Array.prototype.map.call(ele.querySelectorAll(query), function(e) { return e; });
}


/**
 * $$$ returns the DOM element that match the CSS query selector.
 *
 * A shortcut for document.querySelector.
 **/
function $$$(query, ele) {
  if (!ele) {
    ele = document;
  }
  return ele.querySelector(query);
}


this.sk = this.sk || {};

(function(sk) {
  "use strict";

  /**
   * app_config is a place for applications to store app specific
   * configuration variables. If it has been set already, use the existing one.
  **/
  sk.app_config = sk.app_config || {};

  /**
   * clearChildren removes all children of the passed in node.
   */
  sk.clearChildren = function(ele) {
    while (ele.firstChild) {
      ele.removeChild(ele.firstChild);
    }
  }

  /**
   * findParent returns either 'ele' or a parent of 'ele' that has the nodeName of 'nodeName'.
   *
   * Note that nodeName is all caps, i.e. "DIV" or "PAPER-BUTTON".
   *
   * The return value is null if no containing element has that node name.
   */
  sk.findParent = function(ele, nodeName) {
    while (ele != null) {
      if (ele.nodeName == nodeName) {
        return ele;
      }
      ele = ele.parentElement;
    }
    return null;
  }

  /**
   * errorMessage dispatches an event with the error message in it.
   * message is expected to be an object with either a field response
   * (e.g. server response) or message (e.g. message of a typeError)
   * that is a String.
   *
   * See <error-toast-sk> for an element that listens for such events
   * and displays the error messages.
   *
   */
  sk.errorMessage = function(message, duration) {
    if (typeof message === 'object') {
      message = message.response || // for backwards compatibility
          message.message || // for handling Errors {name:String, message:String}
          JSON.stringify(message); // for everything else
    }
    var detail = {
      message: message,
    }
    detail.duration = duration;
    document.dispatchEvent(new CustomEvent('error-sk', {detail: detail, bubbles: true}));
  }

  // elePos returns the position of the top left corner of given element in
  // client coordinates.
  //
  // Returns an object of the form:
  // {
  //   x: NNN,
  //   y: MMM,
  // }
  sk.elePos = function(ele) {
    var bounds = ele.getBoundingClientRect();
    return {x: bounds.left, y: bounds.top};
  }

  // Returns a Promise that uses XMLHttpRequest to make a request with the given
  // method to the given URL with the given headers and body.
  sk.request = function(method, url, body, headers, withCredentials) {
    // Return a new promise.
    return new Promise(function(resolve, reject) {
      // Do the usual XHR stuff
      var req = new XMLHttpRequest();
      req.open(method, url);
      if (headers) {
        for (var k in headers) {
          req.setRequestHeader(k, headers[k]);
        }
      }

      if (withCredentials) {
        req.withCredentials = true;
      }

      req.onload = function() {
        // This is called even on 404 etc
        // so check the status
        if (req.status == 200) {
          // Resolve the promise with the response text
          resolve(req.response);
        } else {
          // Otherwise reject with an object containing the status text and
          // response code, which will hopefully be meaningful error
          reject({
            response: req.response,
            status: req.status,
          });
        }
      };

      // Handle network errors
      req.onerror = function() {
        reject({
            response: Error("Network Error")
          });
      };

      // Make the request
      req.send(body);
    });
  }

  // Returns a Promise that uses XMLHttpRequest to make a request to the given URL.
  sk.get = function(url, withCredentials) {
    return sk.request('GET', url, null, null, withCredentials);
  }


  // Returns a Promise that uses XMLHttpRequest to make a POST request to the
  // given URL with the given JSON body. The content_type is optional and
  // defaults to "application/json".
  sk.post = function(url, body, content_type, withCredentials) {
    if (!content_type) {
      content_type = "application/json";
    }
    return sk.request('POST', url, body, {"Content-Type": content_type}, withCredentials);
  }

  // Returns a Promise that uses XMLHttpRequest to make a DELETE request to the
  // given URL.
  sk.delete = function(url, body, withCredentials) {
    return sk.request('DELETE', url, body, null, withCredentials);
  }

  // A Promise that resolves when DOMContentLoaded has fired.
  sk.DomReady = new Promise(function(resolve, reject) {
      if (document.readyState != 'loading') {
        // If readyState is already past loading then
        // DOMContentLoaded has already fired, so just resolve.
        resolve();
      } else {
        document.addEventListener('DOMContentLoaded', resolve);
      }
    });

  // A Promise that resolves when Polymer has fired polymer-ready.
  sk.WebComponentsReady = new Promise(function(resolve, reject) {
    window.addEventListener('polymer-ready', resolve);
  });

  // _Mailbox is an object that allows distributing, possibly in a time
  // delayed manner, values to subscribers to mailbox names.
  //
  // For example, a series of large objects may need to be distributed across
  // a DOM tree in a way that doesn't easily fit with normal data binding.
  // Instead each element can subscribe to a mailbox name where the data will
  // be placed, and receive a callback when the data is updated. Note that
  // upon first subscribing to a mailbox the callback will be triggered
  // immediately with the value there, which may be the default of null.
  //
  // There is no order required for subscribe and send calls. You can send to
  // a mailbox with no subscribers, and a subscription can be registered for a
  // mailbox that has not been sent any data yet.
  var _Mailbox = function() {
    this.boxes = {};
  };

  // Subscribe to a mailbox of the name 'addr'. The callback 'cb' will
  // be called each time the mailbox is updated, including the very first time
  // a callback is registered, possibly with the default value of null.
  _Mailbox.prototype.subscribe = function(addr, cb) {
    var box = this.boxes[addr] || { callbacks: [], value: null };
    box.callbacks.push(cb);
    cb(box.value);
    this.boxes[addr] = box;
  };

  // Remove a callback from a subscription.
  _Mailbox.prototype.unsubscribe = function(addr, cb) {
    var box = this.boxes[addr] || { callbacks: [], value: null };
    // Use a countdown loop so multiple removals is safe.
    for (var i = box.callbacks.length-1; i >= 0; i--) {
      if (box.callbacks[i] == cb) {
        box.callbacks.splice(i, 1);
      }
    }
  };

  // Send data to a mailbox. All registered callbacks will be triggered
  // synchronously.
  _Mailbox.prototype.send = function(addr, value) {
    var box = this.boxes[addr] || { callbacks: [], value: null };
    box.value = value;
    this.boxes[addr] = box;
    box.callbacks.forEach(function(cb) {
      cb(value);
    });
  };

  // sk.Mailbox is an instance of sk._Mailbox, the only instance
  // that should be needed.
  sk.Mailbox = new _Mailbox();


  // Namespace for utilities for working with human i/o.
  sk.human = {};

  var TIME_DELTAS = [
    { units: "w", delta: 7*24*60*60 },
    { units: "d", delta:   24*60*60 },
    { units: "h", delta:      60*60 },
    { units: "m", delta:         60 },
    { units: "s", delta:          1 },
  ];

  sk.KB = 1024;
  sk.MB = sk.KB * 1024;
  sk.GB = sk.MB * 1024;
  sk.TB = sk.GB * 1024;
  sk.PB = sk.TB * 1024;

  var BYTES_DELTAS = [
    { units: " PB", delta: sk.PB},
    { units: " TB", delta: sk.TB},
    { units: " GB", delta: sk.GB},
    { units: " MB", delta: sk.MB},
    { units: " KB", delta: sk.KB},
    { units: " B",  delta:     1},
  ];

  /**
   * Pad zeros in front of the specified number.
   */
  sk.human.pad = function(num, size) {
    var str = num + "";
    while (str.length < size) str = "0" + str;
    return str;
  }

  /**
   * Returns a human-readable format of the given duration in seconds.
   *
   * For example, 'strDuration(123)' would return "2m 3s".
   *
   * Negative seconds is treated the same as positive seconds.
   */
  sk.human.strDuration = function(seconds) {
    if (seconds < 0) {
      seconds = -seconds;
    }
    if (seconds == 0) { return '  0s'; }
    var rv = "";
    for (var i=0; i<TIME_DELTAS.length; i++) {
      if (TIME_DELTAS[i].delta <= seconds) {
        var s = Math.floor(seconds/TIME_DELTAS[i].delta)+TIME_DELTAS[i].units;
        while (s.length < 4) {
          s = ' ' + s;
        }
        rv += s;
        seconds = seconds % TIME_DELTAS[i].delta;
      }
    }
    return rv;
  };

  /**
   * Returns the difference between the current time and 's' as a string in a
   * human friendly format.
   * If 's' is a number it is assumed to contain the time in milliseconds
   * otherwise it is assumed to contain a time string.
   *
   * For example, a difference of 123 seconds between 's' and the current time
   * would return "2m".
   */
  sk.human.diffDate = function(s) {
    var ms = (typeof(s) == "number") ? s : Date.parse(s);
    var diff = (ms - Date.now())/1000;
    if (diff < 0) {
      diff = -1.0 * diff;
    }
    return humanize(diff, TIME_DELTAS);
  }

  /**
   * Formats the amount of bytes in a human friendly format.
   * unit may be supplied to indicate b is not in bytes, but in something
   * like kilobytes (sk.KB) or megabytes (sk.MB)

   * For example, a 1234 bytes would be displayed as "1 KB".
   */
  sk.human.bytes = function(b, unit) {
    if (Number.isInteger(unit)) {
      b = b * unit;
    }
    return humanize(b, BYTES_DELTAS);
  }

  function humanize(n, deltas) {
    for (var i=0; i<deltas.length-1; i++) {
      // If n would round to '60s', return '1m' instead.
      var nextDeltaRounded =
          Math.round(n/deltas[i+1].delta)*deltas[i+1].delta;
      if (nextDeltaRounded/deltas[i].delta >= 1) {
        return Math.round(n/deltas[i].delta)+deltas[i].units;
      }
    }
    var i = deltas.length-1;
    return Math.round(n/deltas[i].delta)+deltas[i].units;
  }

  // localeTime formats the provided Date object in locale time and appends the timezone to the end.
  sk.human.localeTime = function(date) {
    // caching timezone could be buggy, especially if times from a wide range
    // of dates are used. The main concern would be crossing over Daylight
    // Savings time and having some times be erroneously in EST instead of
    // EDT, for example
    var str = date.toString();
    var timezone = str.substring(str.indexOf("("));
    return date.toLocaleString() + " " + timezone;
  }

  // Gets the epoch time in seconds.  This is its own function to make it easier to mock.
  sk.now = function() {
    return Math.round(new Date().getTime() / 1000);
  }

  // Namespace for utilities for working with arrays.
  sk.array = {};

  /**
   * Returns true if the two arrays are equal.
   *
   * Notes:
   *   Presumes the arrays are already in the same order.
   *   Compares equality using ===.
   */
  sk.array.equal = function(a, b) {
    if (a.length != b.length) {
      return false;
    }
    for (var i = 0, len = a.length; i < len; i++) {
      if (a[i] !== b[i]) {
        return false;
      }
    }
    return true;
  }

  /**
   * Formats the given string, replacing newlines with <br/> and auto-linkifying URLs.
   * References to bugs like "skia:123" and "chromium:123" are also converted into links.
   *
   * If linksInNewWindow is true, links are created with target="_blank".
   */
  sk.formatHTML = function(s, linksInNewWindow) {
    var sub = '<a href="$&">$&</a>';
    if (linksInNewWindow) {
      sub = '<a href="$&" target="_blank">$&</a>';
    }
    s = s.replace(/https?:(\/\/|&#x2F;&#x2F;)[^ \t\n<]*/g, sub).replace(/(?:\r\n|\n|\r)/g, '<br/>');
    return sk.linkifyBugs(s);
  }

  var PROJECTS_TO_ISSUETRACKERS = {
    'chromium': 'http://crbug.com/',
    'skia': 'http://skbug.com/',
  }

  /**
   * Formats bug references like "skia:123" and "chromium:123" into links.
   */
  sk.linkifyBugs = function(s) {
    for (var project in PROJECTS_TO_ISSUETRACKERS) {
      var re = new RegExp(project + ":[0-9]+", "g");
      var found_bugs = s.match(re);
      if (found_bugs) {
        found_bugs.forEach(function(found_bug) {
          var bug_number = found_bug.split(":")[1];
          var bug_link = '<a href="' + PROJECTS_TO_ISSUETRACKERS[project] +
                         bug_number + '" target="_blank">' + found_bug +
                         '</a>';
          s = s.replace(found_bug, bug_link);
        });
      }
    }
    return s;
  }

  // Namespace for utilities for working with URL query strings.
  sk.query = {};


  // fromParamSet encodes an object of the form:
  //
  // {
  //   a:["2", "4"],
  //   b:["3"]
  // }
  //
  // to a query string like:
  //
  // "a=2&a=4&b=3"
  //
  // This function handles URI encoding of both keys and values.
  sk.query.fromParamSet = function(o) {
    if (!o) {
      return "";
    }
    var ret = [];
    var keys = Object.keys(o).sort();
    keys.forEach(function(key) {
      o[key].forEach(function(value) {
        ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(value));
      });
    });
    return ret.join('&');
  }

  // toParamSet parses a query string into an object with
  // arrays of values for the values. I.e.
  //
  //   "a=2&b=3&a=4"
  //
  // decodes to
  //
  //   {
  //     a:["2", "4"],
  //     b:["3"],
  //   }
  //
  // This function handles URI decoding of both keys and values.
  sk.query.toParamSet = function(s) {
    s = s || '';
    var ret = {};
    var vars = s.split("&");
    for (var i=0; i<vars.length; i++) {
      var pair = vars[i].split("=", 2);
      if (pair.length == 2) {
        var key = decodeURIComponent(pair[0]);
        var value = decodeURIComponent(pair[1]);
        if (ret.hasOwnProperty(key)) {
          ret[key].push(value);
        } else {
          ret[key] = [value];
        }
      }
    }
    return ret;
  }


  // fromObject takes an object and encodes it into a query string.
  //
  // The reverse of this function is toObject.
  sk.query.fromObject = function(o) {
    var ret = [];
    Object.keys(o).sort().forEach(function(key) {
      if (Array.isArray(o[key])) {
        o[key].forEach(function(value) {
          ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(value));
        })
      } else if (typeof(o[key]) == 'object') {
          ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(sk.query.fromObject(o[key])));
      } else {
        ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(o[key]));
      }
    });
    return ret.join('&');
  }


  // toObject decodes a query string into an object
  // using the 'target' as a source for hinting on the types
  // of the values.
  //
  //   "a=2&b=true"
  //
  // decodes to:
  //
  //   {
  //     a: 2,
  //     b: true,
  //   }
  //
  // When given a target of:
  //
  //   {
  //     a: 1.0,
  //     b: false,
  //   }
  //
  // Note that a target of {} would decode
  // the same query string into:
  //
  //   {
  //     a: "2",
  //     b: "true",
  //   }
  //
  // Only Number, String, Boolean, Object, and Array of String hints are supported.
  sk.query.toObject = function(s, target) {
    var target = target || {};
    var ret = {};
    var vars = s.split("&");
    for (var i=0; i<vars.length; i++) {
      var pair = vars[i].split("=", 2);
      if (pair.length == 2) {
        var key = decodeURIComponent(pair[0]);
        var value = decodeURIComponent(pair[1]);
        if (target.hasOwnProperty(key)) {
          switch (typeof(target[key])) {
            case 'boolean':
              ret[key] = value=="true";
              break;
            case 'number':
              ret[key] = Number(value);
              break;
            case 'object': // Arrays report as 'object' to typeof.
              if (Array.isArray(target[key])) {
                var r = ret[key] || [];
                r.push(value);
                ret[key] = r;
              } else {
                ret[key] = sk.query.toObject(value, target[key]);
              }
              break;
            case 'string':
              ret[key] = value;
              break;
            default:
              ret[key] = value;
          }
        } else {
          ret[key] = value;
        }
      }
    }
    return ret;
  }

  // splitAmp returns the given query string as a newline
  // separated list of key value pairs. If sepator is not
  // provided newline will be used.
  sk.query.splitAmp = function(queryStr, separator) {
    separator = (separator) ? separator : '\n';
    queryStr = queryStr || "";
    return queryStr.split('&').join(separator);
  };

  // Namespace for utilities for working with Objects.
  sk.object = {};

  // Returns true if a and b are equal, covers Boolean, Number, String and
  // Arrays and Objects.
  sk.object.equals = function(a, b) {
    if (typeof(a) != typeof(b)) {
      return false
    }
    var ta = typeof(a);
    if (ta == 'string' || ta == 'boolean' || ta == 'number') {
      return a === b
    }
    if (ta == 'object') {
      if (Array.isArray(ta)) {
        return JSON.stringify(a) == JSON.stringify(b)
      } else {
        return sk.query.fromObject(a) == sk.query.fromObject(b)
      }
    }
  }

  // Returns an object with only values that are in o that are different
  // from d.
  //
  // Only works shallowly, i.e. only diffs on the attributes of
  // o and d, and only for the types that sk.object.equals supports.
  sk.object.getDelta = function (o, d) {
    var ret = {};
    Object.keys(o).forEach(function(key) {
      if (!sk.object.equals(o[key], d[key])) {
        ret[key] = o[key];
      }
    });
    return ret;
  };

  // Returns a copy of object o with values from delta if they exist.
  sk.object.applyDelta = function (delta, o) {
    var ret = {};
    Object.keys(o).forEach(function(key) {
      if (delta.hasOwnProperty(key)) {
        ret[key] = JSON.parse(JSON.stringify(delta[key]));
      } else {
        ret[key] = JSON.parse(JSON.stringify(o[key]));
      }
    });
    return ret;
  };

  // Returns a shallow copy (top level keys) of the object.
  sk.object.shallowCopy = function(o) {
    var ret = {};
    for(var k in o) {
      if (o.hasOwnProperty(k)) {
        ret[k] = o[k];
      }
    }
    return ret;
  };

  // Namespace for utilities for working with structured keys.
  //
  // See /go/query for a description of structured keys.
  sk.key = {};

  // Returns true if paramName=paramValue appears in the given structured key.
  sk.key.matches = function(key, paramName, paramValue) {
    return key.indexOf("," + paramName + "=" + paramValue + ",") >= 0;
  };

  // Parses the structured key and returns a populated object with all
  // the param names and values.
  sk.key.toObject = function(key) {
    var ret = {};
    key.split(",").forEach(function(s, i) {
      if (i == 0 ) {
        return
      }
      if (s === "") {
        return;
      }
      var parts = s.split("=");
      if (parts.length != 2) {
        return
      }
      ret[parts[0]] = parts[1];
    });
    return ret;
  };

  // Track the state of a page and reflect it to and from the URL.
  //
  // page - An object with a property 'state' where the state to be reflected
  //        into the URL is stored. We need the level of indirection because
  //        JS doesn't have pointers.
  //
  //        The 'state' must be on Object and all the values in the Object
  //        must be Number, String, Boolean, Object, or Array of String.
  //        Doesn't handle NaN, null, or undefined.
  // cb   - A callback of the form function() that is called when state has been
  //        changed by a change in the URL.
  sk.stateReflector = function(page, cb) {
    // The default state of the page. Used to calculate diffs to state.
    var defaultState = JSON.parse(JSON.stringify(page.state));

    // The last state of the page. Used to determine if the page state has changed recently.
    var lastState = JSON.parse(JSON.stringify(page.state));

    // Watch for state changes and reflect them in the URL by simply
    // polling the object and looking for differences from defaultState.
    setInterval(function() {
      if (Object.keys(sk.object.getDelta(lastState, page.state)).length > 0) {
        lastState = JSON.parse(JSON.stringify(page.state));
        var q = sk.query.fromObject(sk.object.getDelta(page.state, defaultState));
        history.pushState(null, "", window.location.origin + window.location.pathname + "?" +  q);
      }
    }, 100);

    // stateFromURL should be called when the URL has changed, it updates
    // the page.state and triggers the callback.
    var stateFromURL = function() {
      var delta = sk.query.toObject(window.location.search.slice(1), defaultState);
      page.state = sk.object.applyDelta(delta, defaultState);
      lastState = JSON.parse(JSON.stringify(page.state));
      cb();
    }

    // When we are loaded we should update the state from the URL.
    //
    // We check to see if we are running Polymer 0.5, in which case
    // we need to wait for Polymer to finish initializing, otherwise
    // we can just wait for DomReady.
    if (window["Polymer"] && Polymer.version[0] == "0") {
      sk.WebComponentsReady.then(stateFromURL);
    } else {
      sk.DomReady.then(stateFromURL);
    }

    // Every popstate event should also update the state.
    window.addEventListener('popstate', stateFromURL);
  }

  // Find a "round" number in the given range. Attempts to find numbers which
  // consist of a multiple of one of the following, order of preference:
  // [5, 2, 1], followed by zeroes.
  //
  // TODO(borenet): It would be nice to support other multiples, for example,
  // when dealing with time data, it'd be nice to round to seconds, minutes,
  // hours, days, etc.
  sk.getRoundNumber = function(min, max, base) {
    if (min > max) {
      throw ("sk.getRoundNumber: min > max! (" + min + " > " + max + ")");
    }
    var multipleOf = [5, 2, 1];

    var val = (max + min) / 2;
    // Determine the number of digits left of the decimal.
    if (!base) {
      base = 10;
    }
    var digits = Math.floor(Math.log(Math.abs(val)) / Math.log(base)) + 1;

    // Start with just the most significant digit and attempt to round it to
    // multiples of the above numbers, gradually including more digits until
    // a "round" value is found within the given range.
    for (var shift = 0; ; shift++) {
      // Round by shifting digits and dividing by a multiplier, then performing
      // the round function, then multiplying and shifting back.
      var shiftDiv = Math.pow(base, (digits - shift));
      for (var i = 0; i < multipleOf.length; i++) {
        var f = shiftDiv * multipleOf[i];
        // Actually perform the rounding. The 10s are included to intentionally
        // reduce precision to round off floating point error.
        var newVal = ((Math.round(val / f) * 10) * f) / 10;
        if (newVal >= min && newVal <= max) {
          return newVal;
        }
      }
    }

    console.error("sk.getRoundNumber Couldn't find appropriate rounding " +
                  "value. Returning midpoint.");
    return val;
  }

  // Sort the given array of strings, ignoring case.
  sk.sortStrings = function(s) {
    return s.sort(function(a, b) {
      return a.localeCompare(b, "en", {"sensitivity": "base"});
    });
  }

  // Capitalize each word in the string.
  sk.toCapWords = function(s) {
    return s.replace(/\b\w/g, function(firstLetter) {
      return firstLetter.toUpperCase();
    });
  }

  // Truncate the given string to the given length. If the string was
  // shortened, change the last three characters to ellipsis.
  sk.truncate = function(str, len) {
    if (str.length > len) {
      var ellipsis = "..."
      return str.substring(0, len - ellipsis.length) + ellipsis;
    }
    return str
  }

  // Return a 32 bit hash for the given string.
  //
  // This is a super simple hash (h = h * 31 + x_i) currently used
  // for things like assigning colors to graphs based on trace ids. It
  // shouldn't be used for anything more serious than that.
  sk.hashString = function(s) {
    var hash = 0;
    for (var i = s.length - 1; i >= 0; i--) {
      hash = ((hash << 5) - hash) + s.charCodeAt(i);
      hash |= 0;
    }
    return Math.abs(hash);
  }

  // Returns the string with all instances of &,<,>,",',/
  // replaced with their html-safe equivilents.
  // See OWASP doc https://goto.google.com/hyaql
  sk.escapeHTML = function(s) {
    return s.replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;')
    .replace(/\//g, '&#x2F;');

  }

  // Returns true if the sorted arrays a and b
  // contain at least one element in common
  sk.sharesElement = function(a, b) {
    var i = 0;
    var j = 0;
    while (i < a.length && j < b.length) {
      if (a[i] < b[j]) {
        i++;
      } else if (b[j] < a[i]) {
        j++;
      } else {
        return true;
      }
    }
    return false;
  }


  // robust_get finds a sub object within 'obj' by following the path
  // in 'idx'. It will not throw an error if any sub object is missing
  // but instead return 'undefined'. 'idx' has to be an array.
  sk.robust_get = function(obj, idx) {
    if (!idx || !obj) {
      return;
    }

    for(var i=0, len=idx.length; i<len; i++) {
      if ((typeof obj === 'undefined') || (typeof idx[i] === 'undefined')) {
        return;  // returns 'undefined'
      }

      obj = obj[idx[i]];
    }

    return obj;
  };

  // Utility function for colorHex.
  function _hexify(i) {
    var s = i.toString(16).toUpperCase();
    // Pad out to two hex digits if necessary.
    if (s.length < 2) {
      s = '0' + s;
    }
    return s;
  }

  // colorHex returns a hex representation of a given color pixel as a string.
  // 'colors' is an array of bytes that contain pixesl in  RGBA format.
  // 'offset' is the offset of the pixel of interest.
  sk.colorHex = function(colors, offset) {
    return '#'
      + _hexify(colors[offset+0])
      + _hexify(colors[offset+1])
      + _hexify(colors[offset+2])
      + _hexify(colors[offset+3]);
  };

  // colorRGB returns the given RGBA pixel as a 4-tupel of decimal numbers.
  // 'colors' is an array of bytes that contain pixesl in  RGBA format.
  // 'offset' is the offset of the pixel of interest.
  // 'rawAlpha' will return the alpha value directly if true. Otherwise it will
  //            be normalized to [0...1].
  sk.colorRGB = function(colors, offset, rawAlpha) {
    var scaleAlpha = (rawAlpha) ? 1 : 255;
    return "rgba(" + colors[offset] + ", " +
              colors[offset + 1] + ", " +
              colors[offset + 2] + ", " +
              colors[offset + 3] / scaleAlpha + ")";
  };

  // Polyfill for String.startsWith from
  // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/String/startsWith#Polyfill
  // returns true iff the string starts with the given prefix
  if (!String.prototype.startsWith) {
    String.prototype.startsWith = function(searchString, position) {
      position = position || 0;
      return this.indexOf(searchString, position) === position;
    };
  }

})(sk);
