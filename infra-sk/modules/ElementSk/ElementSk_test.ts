import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { assert } from 'chai';
import { ElementSk } from './ElementSk';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

class MyTestElementSk extends ElementSk {
  attributeCalled = false;

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
    this.attributeCalled = true;
  }
}

define('my-test-element-sk', MyTestElementSk);

describe('ElementSk', () => {
  describe('render', () => {
    it('only renders if connected', () => {
      container.innerHTML = '<my-test-element-sk some-attribute><my-test-element-sk>';
      const ele = container.firstElementChild as MyTestElementSk;
      assert.isNotNull(ele.querySelector('p'));
      assert.isTrue(ele.attributeCalled);
    });
  });
});
