export class ParamSet {
  constructor(ignored = []) {
    this._ps = {};
    this._ignored = new Set(ignored);
  }

  add(params) {
    for (let key in params) {
      if (this._ignored.has(key)) {
        continue
      }
      let set = this._ps[key] || new Set();
      set.add(params[key]);
      this._ps[key] = set;
    }
  }

  match(params) {
    for (let key in this._ps) {
      if (!this._ps[key].has(params[key])) {
        return false
      }
    }
    return true
  }
}

