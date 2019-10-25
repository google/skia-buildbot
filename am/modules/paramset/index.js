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
    console.log('--------')
    console.log(paramset)
    console.log(params)
    console.log('--------')
    if (paramset[key].includes(params[key])) {
      continue;
    }
    const values = paramset[key] || [];
    for (let i in values) {
      var re = new RegExp("^" + values[i] + "$")
      if (!re.test(params[key])) {
        console.log(key)
        console.log(params[key])
        console.log(values[i])
        console.log(re)
        console.log(re.test(params[key]))
        return false
      }
    }
    //if (!paramset[key].includes(params[key])) {
    //  return false
    //}
  }
  return true
}

/**
 * Similar to match but with regex support.
 */
export function matchWithRegex(paramset, params) {
  // console.log("/^abc123$/".test('abc123')); // true

}
