// This module contains functions for DOM manipulation.

// $ returns a real JS array of DOM elements that match the CSS query selector.
export const $ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};

// $$ returns the first DOM element that matches the CSS query selector.
export const $$ = (query, ele = document) => ele.querySelector(query);

// Capture the value from the unupgraded instance and delete the property so
// it does not shadow the custom element's own property setter.
//
// See the following for more details:
// https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties
export function upgradeProperty(ele, prop) {
  if (ele.hasOwnProperty(prop)) {
    let value = ele[prop];
    delete ele[prop];
    ele[prop] = value;
  }
}
