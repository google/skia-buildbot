/** @module common-sk/modules/dom */
/**
 * A Promise that resolves when DOMContentLoaded has fired.
 */
export const DomReady = new Promise(function(resolve, reject) {
  if (document.readyState !== 'loading') {
    // If readyState is already past loading then
    // DOMContentLoaded has already fired, so just resolve.
    resolve();
  } else {
    document.addEventListener('DOMContentLoaded', resolve);
  }
});

/** @function $
 *
 * @description Returns a real JS array of DOM elements that match the CSS selector.
 *
 * @param {string} query CSS selector string.
 * @param {Element} ele The Element to start the search from.
 * @returns {Array} Array of DOM Elements that match the CSS selector.
 *
 */
export const $ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};

/** @function $$
 *
 * @description Returns the first DOM element that matches the CSS query selector.
 *
 * @param {string} query CSS selector string.
 * @param {Element} ele The Element to start the search from.
 * @returns {Element} The first Element in DOM order that matches the CSS selector.
 */
export const $$ = (query, ele = document) => ele.querySelector(query);

