// A map with a default value. Great for counting or binnings things by key.

export class DefaultMap<K, V> extends Map<K, V> {
  private initfn: ()=> V;

  // Accepts a function that returns the dict's default value, an empty Array for example.
  constructor(fn: ()=> V) {
    super();
    this.initfn = fn;
  }

  get(key: K): V {
    if (!this.has(key)) {
      this.set(key, this.initfn());
    }
    return super.get(key)!;
  }
}
