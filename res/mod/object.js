import { fromObject } from './query.js'

// Makes a deep copy of an object.
export const deepCopy = (o) => JSON.parse(JSON.stringify(o));

// Returns true if a and b are equal, covers Boolean, Number, String and
// Arrays and Objects.
export function equals(a, b) {
  if (typeof(a) != typeof(b)) {
    return false
  }
  let ta = typeof(a);
  if (ta == 'string' || ta == 'boolean' || ta == 'number') {
    return a === b
  }
  if (ta == 'object') {
    if (Array.isArray(ta)) {
      return JSON.stringify(a) == JSON.stringify(b)
    } else {
      return fromObject(a) == fromObject(b)
    }
  }
}

// Returns an object with only values that are in o that are different
// from d.
//
// Only works shallowly, i.e. only diffs on the attributes of
// o and d, and only for the types that equals() supports.
export function getDelta(o, d) {
    let ret = {};
    Object.keys(o).forEach(function(key) {
      if (!equals(o[key], d[key])) {
        ret[key] = o[key];
      }
    });
    return ret;
  };

// Returns a copy of object o with values from delta if they exist.
export function applyDelta(delta, o) {
  let ret = {};
  Object.keys(o).forEach(function(key) {
    if (delta.hasOwnProperty(key)) {
      ret[key] = deepCopy(delta[key]);
    } else {
      ret[key] = deepCopy(o[key]);
    }
  });
  return ret;
};

