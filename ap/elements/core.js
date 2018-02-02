export var $ = (id, ele = document) => ele.getElementById(id);
export var $$ = (query, ele = document) => {
  return Array.prototype.map.call(ele.querySelectorAll(query), (e) => e);
};
