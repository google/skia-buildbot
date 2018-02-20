import './index.js'

let ee = document.querySelector('example-element');

document.querySelector('button').addEventListener('click', (e) => {
  ee.active = !ee.active;
});
