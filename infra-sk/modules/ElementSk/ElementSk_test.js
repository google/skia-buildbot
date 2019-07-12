import { ElementSk } from './ElementSk.js'
import { html } from 'lit-html'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

window.customElements.define('my-test-element-sk', class extends ElementSk {
  constructor() {
    super();
    this._template = (ele) => html`<p>Hello World!</p>`;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});

describe('ElementSk', function() {
  describe('render', function() {
    it('only renders if connected', function() {
      return window.customElements.whenDefined('my-test-element-sk').then(() => {
        container.innerHTML = `<my-test-element-sk><my-test-element-sk>`;
        let ele = container.firstElementChild;
        let p = ele.querySelector('p')
        assert.isNotNull(p);
      })
    });
  });
});
