import { $, upgradeProperty } from '../dom';

// TODO(jcgregorio) Currently only sets the selected attribute on the next
// sibling if the next sibling is a 'tabs-panel-sk'. We should also have
// the ability to set the id of the 'tabs-panel-sk' we want to affect.

// The <tabs-sk> custom element declaration, used in conjunction
// with <button>'s and the <tabs-panel-sk> element allows you to
// create tabbed interfaces. The association between the buttons
// and the tabs displayed in tabs-panel-sk is document order,
// i.e. the first button shows the first panel, second button shows
// second panel, etc.
//
//      <tabs-sk>
//        <button class=selected>Query</button>
//        <button>Results</button>
//      </tabs-sk>
//      <tabs-panel-sk>
//        <div>
//          This is the query tab.
//        </div>
//        <div>
//          This is the results tab.
//        </div>
//      </tabs-panel-sk>
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    tab-selected-sk - Event sent when the user clicks on a tab. The events
//        value of detail.index is the index of the selected tab.
//
//  Methods:
//    select(n) - Forces the selection of the 'n'th panel.
//
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

// The <tabs-panel-sk> custom element declaration.
//
//  Attributes:
//    selected - The index of the tab panel to display.
//
//  Properties:
//    selected - Mirrors the 'selected' attribute.
//
//  Events:
//    None
//
//  Methods:
//    None
//
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
    this._select(val);
  }

	attributeChangedCallback(name, oldValue, newValue) {
    this._select(+newValue);
	}

  _select(index) {
    for (let i=0; i<this.children.length; i++) {
      this.children[i].classList.toggle('selected', i === index);
    }
  }
});
