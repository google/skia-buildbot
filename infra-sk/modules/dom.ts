// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/** @module common-sk/modules/dom */
/**
 * A Promise that resolves when DOMContentLoaded has fired.
 */
export const DomReady = new Promise((resolve) => {
  if (document.readyState !== 'loading') {
    // If readyState is already past loading then
    // DOMContentLoaded has already fired, so just resolve.
    resolve(undefined);
  } else {
    document.addEventListener('DOMContentLoaded', resolve);
  }
});

/** @function $
 *
 * @description Returns a real JS array of DOM elements that match the CSS selector.
 *
 * @param query CSS selector string.
 * @param ele The Element to start the search from.
 * @returns Array of DOM Elements that match the CSS selector.
 *
 */
export function $<E extends Element = Element>(
  query: string,
  ele: Element | Document = document
): E[] {
  return Array.from(ele.querySelectorAll<E>(query));
}

/** @function $$
 *
 * @description Returns the first DOM element that matches the CSS query selector.
 *
 * @param query CSS selector string.
 * @param ele The Element to start the search from.
 * @returns The first Element in DOM order that matches the CSS selector.
 */
export function $$<E extends Element = Element>(
  query: string,
  ele: Element | Document = document
): E | null {
  return ele.querySelector(query);
}

/**
 * Find the first parent of 'ele' with the given 'nodeName'.
 *
 * @param ele - The element to start searching a.
 * @param nodeName - The node name we are looking for.
 * @returns Either 'ele' or the first parent of 'ele' that has the nodeName of 'nodeName'. Returns
 *   null if none are found.
 *
 * @example
 *
 *   findParent(ele, 'DIV')
 *
 */
export function findParent(
  ele: HTMLElement | null,
  nodeName: string
): HTMLElement | null {
  while (ele !== null) {
    if (ele.nodeName === nodeName) {
      return ele;
    }
    ele = ele.parentElement;
  }
  return null;
}

/**
 * Find the first parent of 'ele' with the given 'nodeName'. Just like findParent, but TypeScript
 * typesafe.
 *
 * @param ele - The element to start searching a.
 * @param nodeName - The lower-case node name we are looking for, e.g. 'div'.
 * @returns Either 'ele' or the first parent of 'ele' that has the nodeName of 'nodeName'. Returns
 *   null if none are found.
 *
 * @example
 *
 *   findParentSafe(ele, 'div')
 *
 */
export function findParentSafe<K extends keyof HTMLElementTagNameMap>(
  ele: HTMLElement | null,
  nodeName: K
): HTMLElementTagNameMap[K] | null {
  while (ele !== null) {
    if (ele.nodeName.toLowerCase() === nodeName) {
      return ele as HTMLElementTagNameMap[K];
    }
    ele = ele.parentElement;
  }
  return null;
}
