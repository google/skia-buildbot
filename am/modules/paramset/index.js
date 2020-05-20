/**
 * Add some params to a ParamSet.
 *
 * @param {Object} paramset - Key-value pairs where keys are strings and values are arrays of strings.
 * @param {Object} params - Key-value pairs where both keys and values are strings.
 * @param {Array} ignored - List of keys to ignore.
 */
export function add(paramset, params, ignored = []) {
  for (const key in params) {
    if (ignored.includes(key)) {
      continue;
    }
    const value = params[key];
    const values = paramset[key] || [];
    if (!values.includes(value)) {
      values.push(value);
      paramset[key] = values;
    }
  }
}

/**
 * Determines if the params given match the ParamSet.
 *
 * @param {Object} paramset - Key-value pairs where keys are strings and values are arrays of strings
 *   or regexes (see skbug.com/9587).
 * @param {Object} params - Key-value pairs where both keys and values are strings.
 *
 * @return {bool} True if every key in ParamSet is present in 'params' and
 *   the value seen in params is in the ParamSet values or it matches one of
 *   the regexes.
 */
export function match(paramset, params) {
  for (const key in paramset) {
    if (paramset[key].includes(params[key])) {
      continue;
    }
    const values = paramset[key] || [];
    let valMatched = false;
    for (const i in values) {
      const re = new RegExp(`^${values[i]}$`);
      if (re.test(params[key])) {
        valMatched = true;
        break;
      }
    }
    if (!valMatched) {
      return false;
    }
  }
  return true;
}
