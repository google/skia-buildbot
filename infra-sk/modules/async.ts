// This module contains async versions of various built-in JavaScript functions and methods.

/** Async version of Array.prototype.find(), where the callback function returns a promise. */
export async function asyncFind<T>(
    items: T[] | Promise<T[]> | undefined,
    predicate: (needle: T) => Promise<boolean>): Promise<T | undefined> {
  if (!items) {
    return undefined;
  }
  const actualItems = items instanceof Promise ? await items : items;
  for (const item of actualItems) {
    if (await (predicate(item))) {
      return item;
    }
  }
  return undefined;
}

/** Async version of Array.prototype.filter(), where the callback function returns a promise. */
export async function asyncFilter<T>(
    items: T[] | Promise<T[]> | undefined,
    predicate: (needle: T) => Promise<boolean>): Promise<T[]> {
  if (!items) {
    return [];
  }
  const actualItems = items instanceof Promise ? await items : items;
  const filteredItems: T[] = [];
  for (const item of actualItems) {
    if (await (predicate(item))) {
      filteredItems.push(item);
    }
  }
  return filteredItems;
}

/** Async version of Array.prototype.map(), where the callback function returns a promise. */
export async function asyncMap<F, T>(
    items: F[] | Promise<F[]> | undefined, fn: (from: F) => Promise<T>): Promise<T[]> {
  if (!items) {
    return [];
  }
  const actualItems = items instanceof Promise ? await items : items;
  return Promise.all(actualItems.map(fn));
}

/** Async version of Array.prototype.forEach(), where the callback function returns a promise. */
export async function asyncForEach<T>(
    items: T[] | Promise<T[]> | undefined, fn: (val: T) => Promise<void>): Promise<void> {
  await asyncMap(items, fn);
}
