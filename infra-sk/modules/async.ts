// This module contains async versions of various built-in JavaScript functions and methods.

/** Async version of Array.prototype.find(), where the callback function returns a promise. */
export async function asyncFind<T>(
    haystack: T[] | Promise<T[]> | undefined,
    predicate: (needle: T) => Promise<boolean>): Promise<T | undefined> {
  if (!haystack) {
    return undefined;
  }
  const actualHaystack = haystack instanceof Promise ? await haystack : haystack;
  for (const needle of actualHaystack) {
    if (await (predicate(needle))) {
      return needle;
    }
  }
  return undefined;
}

/** Async version of Array.prototype.map(), where the callback function returns a promise. */
export async function asyncMap<F, T>(
    input: F[] | Promise<F[]> | undefined, fn: (from: F) => Promise<T>): Promise<T[]> {
  if (!input) {
    return [];
  }
  const actualInput = input instanceof Promise ? await input : input;
  return Promise.all(actualInput.map(fn));
}
