export class ParamSet {
  constructor() {
    this._ps = {};
  }

  add(params) {
    for (let key in params) {
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

