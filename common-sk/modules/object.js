/** @module common-sk/modules/object
 *  @description Utility functions for dealing with Objects.
 */
import { fromObject } from './query.js'

/** @method deepCopy
 *  @param {Object} object - The object to make a copy of.
 *  @returns {Object}
 */
export const deepCopy = (o) => JSON.parse(JSON.stringify(o));

/** Returns true if a and b are equal, covers Boolean, Number, String and Arrays and Objects.
 *
 * @param {(number|boolean|string|Array|Object)} a
 * @param {(number|boolean|string|Array|Object)} b
 * @returns {boolean} True if the arguments are equal.
 */
export function equals(a, b) {
  if (typeof(a) !== typeof(b)) {
    return false
  }
  let ta = typeof(a);
  if (ta === 'string' || ta === 'boolean' || ta === 'number') {
    return a === b
  }
  if (ta === 'object') {
    if (Array.isArray(ta)) {
      return JSON.stringify(a) === JSON.stringify(b)
    } else {
      return fromObject(a) === fromObject(b)
    }
  }
}

/** Returns an object with only values that are in o that are different from d.
 *
 * Only works shallowly, i.e. only diffs on the attributes of
 * o and d, and only for the types that equals() supports.
 *
 * @example
 * // Returns {a:2}
 * getDelta({a:2, b:"foo"}, {a:1, b:"foo", c:3.14})
 *
 * @param {Object} o
 * @param {Object} d
 * @returns {Object}
 *
 */
export function getDelta(o, d) {
    let ret = {};
    Object.keys(o).forEach(function(key) {
      if (!equals(o[key], d[key])) {
        ret[key] = o[key];
      }
    });
    return ret;
  };

/** Returns a copy of object o with values from delta if they exist.
 *
 * @param {Object} delta - A delta object as returned from 'getDelta'.
 * @param {Object} o
 * @returns {Object}
 *
 */
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

