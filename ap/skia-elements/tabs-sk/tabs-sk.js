import { $, upgradeProperty } from '../dom';

// TODO(jcgregorio) Currently only sets the selected attribute on the next
// sibling if the next sibling is a 'tabs-panel-sk'. We should also have
// the ability to set the id of the 'tabs-panel-sk' we want to affect.
//
// TODO(jcgregorio) Enable keyboard nav and proper tabindex behavior ala
// radio-group-sk, along with focus indicators.
window.customElements.define('tabs-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this.addEventListener('click', this);
    this.select(0, false)
  }

  disconnectedCallback() {
    this.removeEventListener('click', this);
  }

  handleEvent(e) {
    e.stopPropagation();
    $('button', this).forEach((ele, i) => {
      if (ele === e.target) {
        ele.classList.add('selected');
        this._trigger(i, true);
      } else {
        ele.classList.remove('selected');
      }
    });
  }

  select(index, trigger) {
    $('button', this).forEach((ele, i) => {
      ele.classList.toggle('selected', i === index);
    });
    this._trigger(index, trigger);
  }

  _trigger(index, trigger) {
    if (trigger) {
      this.dispatchEvent(new CustomEvent('tab-selected-sk', { bubbles: true, detail: { index: index }}));
    }
    if (this.nextElementSibling.tagName === 'TABS-PANEL-SK') {
      this.nextElementSibling.setAttribute('selected', index);
    }
  }
});

window.customElements.define('tabs-panel-sk', class extends HTMLElement {
  static get observedAttributes() {
    return ['selected'];
  }

  connectedCallback() {
    upgradeProperty(this, 'selected');
  }

  get selected() { return this.hasAttribute('selected'); }
  set selected(val) {
    this.setAttribute('selected', val);
    this.select(val);
  }

	attributeChangedCallback(name, oldValue, newValue) {
    this.select(+newValue);
	}

  select(index) {
    for (let i=0; i<this.children.length; i++) {
      this.children[i].classList.toggle('selected', i === index);
    }
  }
});
