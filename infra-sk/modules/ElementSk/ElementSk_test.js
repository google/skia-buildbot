import { define } from 'elements-sk/define'
import { ElementSk } from './ElementSk.js'
import { html } from 'lit-html'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

define('my-test-element-sk', class extends ElementSk {
  constructor() {
    super((ele) => html`<p>Hello World!</p>`);
    assert.isNull(this.querySelector('p'));
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  static get observedAttributes() {
    return ['some-attribute'];
  }

  attributeChangedCallback() {
    super._render();
    assert.isFalse(this._connected);
    assert.isNull(this.querySelector('p'));
    this._attributeCalled = true;
  }
});

describe('ElementSk', function() {
  describe('render', function() {
    it('only renders if connected', function() {
        container.innerHTML = `<my-test-element-sk some-attribute><my-test-element-sk>`;
        let ele = container.firstElementChild;
        assert.isNotNull(ele.querySelector('p'));
        assert.isTrue(ele._attributeCalled);
    });
  });
});


