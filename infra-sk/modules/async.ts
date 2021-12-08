// This module contains async versions of various built-in JavaScript functions and methods.

/** Async version of Array.prototype.find(), where the callback function returns a promise. */
export async function asyncFind<T>(
  items: T[] | Promise<T[]>,
  predicate: (item: T, index: number)=> Promise<boolean>,
): Promise<T | null> {
  if (!items) {
    return null;
  }
  const actualItems = items instanceof Promise ? await items : items;
  for (let i = 0; i < actualItems.length; i++) {
    // eslint-disable-next-line no-await-in-loop
    if (await (predicate(actualItems[i], i))) {
      return actualItems[i];
    }
  }
  return null;
}

/** Async version of Array.prototype.filter(), where the callback function returns a promise. */
export async function asyncFilter<T>(
  items: T[] | Promise<T[]>,
  predicate: (item: T, index: number)=> Promise<boolean>,
): Promise<T[]> {
  if (!items) {
    return [];
  }
  const actualItems = items instanceof Promise ? await items : items;
  const filteredItems: T[] = [];
  for (let i = 0; i < actualItems.length; i++) {
    // eslint-disable-next-line no-await-in-loop
    if (await (predicate(actualItems[i], i))) {
      filteredItems.push(actualItems[i]);
    }
  }
  return filteredItems;
}

/** Async version of Array.prototype.map(), where the callback function returns a promise. */
export async function asyncMap<F, T>(
  items: F[] | Promise<F[]>,
  fn: (from: F, index: number)=> Promise<T>,
): Promise<T[]> {
  if (!items) {
    return [];
  }
  const actualItems = items instanceof Promise ? await items : items;
  return Promise.all(actualItems.map(fn));
}

/** Async version of Array.prototype.forEach(), where the callback function returns a promise. */
export async function asyncForEach<T>(
  items: T[] | Promise<T[]>,
  fn: (item: T, index: number)=> Promise<void>,
): Promise<void> {
  await asyncMap(items, fn);
}
