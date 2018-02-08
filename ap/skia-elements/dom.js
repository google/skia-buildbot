// This module contains functions for DOM manipulation.

// $ returns the element that has the given id in the document.
export var $ = (id, ele = document) => ele.getElementById(id);

// $$ returns a real JS array of DOM elements that match the CSS query selector.
export var $$ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};

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
