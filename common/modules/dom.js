// A Promise that resolves when DOMContentLoaded has fired.
export const DomReady = new Promise(function(resolve, reject) {
  if (document.readyState !== 'loading') {
    // If readyState is already past loading then
    // DOMContentLoaded has already fired, so just resolve.
    resolve();
  } else {
    document.addEventListener('DOMContentLoaded', resolve);
  }
});

// $ returns a real JS array of DOM elements that match the CSS query selector.
export const $ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};

// $$ returns the first DOM element that matches the CSS query selector.
export const $$ = (query, ele = document) => ele.querySelector(query);

