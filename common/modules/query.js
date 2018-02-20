// Utilities for working with URL query strings.

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
export function fromParamSet(o) {
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
export function toParamSet(s) {
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
export function fromObject(o) {
  var ret = [];
  Object.keys(o).sort().forEach(function(key) {
    if (Array.isArray(o[key])) {
      o[key].forEach(function(value) {
        ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(value));
      })
    } else if (typeof(o[key]) == 'object') {
      ret.push(encodeURIComponent(key) + '=' + encodeURIComponent(fromObject(o[key])));
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
export function toObject(s, target) {
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
              ret[key] = toObject(value, target[key]);
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
export function splitAmp(queryStr, separator) {
  separator = (separator) ? separator : '\n';
  queryStr = queryStr || "";
  return queryStr.split('&').join(separator);
};
