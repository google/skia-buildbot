/** Class representing keys and multiple string values per key. */
export class ParamSet {
  /**
   * Create a ParamSet.
   *
   * @param {Array} ignored - A list of keys to ignore, i.e. they won't be added to the ParamSet.
   */
  constructor(ignored = []) {
    this._ps = {};
    this._ignored = new Set(ignored);
  }

  /**
   * Add some params to the ParamSet.
   *
   * @param {Object} params - Key-value pairs where both keys and values are strings.
   */
  add(params) {
    for (let key in params) {
      if (this._ignored.has(key)) {
        continue
      }
      let value = params[key];
      let values = this._ps[key] || [];
      if (!values.includes(value)) {
        values.push(value);
        this._ps[key] = values;
      }
    }
  }

  /**
   * Determines if the params given match the ParamSet.
   *
   * @param {Object} params - Key-value pairs where both keys and values are strings.
   * @return {bool} True if every key in ParamSet is present in 'params' and
   *   the value seen in params is in the ParamSet values.
   */
  match(params) {
    for (let key in this._ps) {
      if (!this._ps[key].includes(params[key])) {
        return false
      }
    }
    return true
  }

  /**
   * Returns the underlying data structure of the ParamSet, a map of strings
   * to arrays of strings.
   *
   * @return {Object} The data in the ParamSet.
   */
  value() {
    return this._ps;
  }
}

