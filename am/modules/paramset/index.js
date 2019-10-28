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
 * @param {String} regexMarker - Which prefix to look for in values to determine if they are regexes.
 *
 * @return {bool} True if every key in ParamSet is present in 'params' and
 *   the value seen in params is in the ParamSet values.
 */
export function match(paramset, params, regexMarker) {
  for (let key in paramset) {
    /*
    console.log('--------')
    console.log(paramset)
    console.log(params)
    console.log('--------')
    */
    const values = paramset[key] || [];
    let regexFound = false;
    for (let i in values) {
      if (values[i].startsWith(regexMarker)) {
        regexFound = true;
        // do the regex processing here.
        var re = new RegExp("^" + values[i] + "$")
        console.log("FOUND REGEX!");
        if (!re.test(params[key])) {
          console.log(key)
          console.log(params[key])
          console.log(values[i])
          console.log(re)
          console.log(re.test(params[key]))
          return false
        }
      }
    }
    if (!regexFound && !paramset[key].includes(params[key])) {
      return false;
    }
  }
  return true
}

/**
 * Similar to match but with regex support.
 */
export function matchWithRegex(paramset, params) {
  // console.log("/^abc123$/".test('abc123')); // true

}
