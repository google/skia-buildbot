export var $ = (id, ele = document) => ele.getElementById(id);

// $$ returns a real JS array of DOM elements that match the CSS query selector.
export var $$ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};
