/**
 * Add some params to a ParamSet.
 *
 * @param {Object} paramset - Key-value pairs where keys are strings and values are arrays of strings.
 * @param {Object} params - Key-value pairs where both keys and values are strings.
 * @param {Array} ignored - List of keys to ignore.
 */
export function add(paramset, params, ignored = []) {
  for (let key in params) {
    if (ignored.includes(key)) {
      continue
    }
    let value = params[key];
    let values = paramset[key] || [];
    if (!values.includes(value)) {
      values.push(value);
      paramset[key] = values;
    }
  }
}

/**
 * Determines if the params given match the ParamSet.
 *
 * @param {Object} paramset - Key-value pairs where keys are strings and values are arrays of strings.
 * @param {Object} params - Key-value pairs where both keys and values are strings.
 *
 * @return {bool} True if every key in ParamSet is present in 'params' and
 *   the value seen in params is in the ParamSet values.
 */
export function match(paramset, params) {
  for (let key in paramset) {
    if (!paramset[key].includes(params[key])) {
      return false
    }
  }
  return true
}

