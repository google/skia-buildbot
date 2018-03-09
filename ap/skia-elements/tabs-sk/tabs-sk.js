/** @module skia-elements/tabs-sk */
import { upgradeProperty } from '../upgradeProperty';

// TODO(jcgregorio) Currently only sets the selected attribute on the next
// sibling if the next sibling is a 'tabs-panel-sk'. We should also have
// the ability to set the id of the 'tabs-panel-sk' we want to affect.

/**
 * <code>tabs-sk</code>
 *
 * <p>
 * The tabs-sk custom element declaration, used in conjunction with button and
 * the [tabs-panel-sk]{@link module:skia-elements/tabs-sk~TabsPanelSk} element
 * allows you to create tabbed interfaces. The association between the buttons
 * and the tabs displayed in [tabs-panel-sk]{@link module:skia-elements/tabs-sk~TabsPanelSk}
 * is document order, i.e. the first button shows the first panel, second
 * button shows second panel, etc.
 * </p>
 *
 * @example
 *
 * <tabs-sk>
 *   <button class=selected>Query</button>
 *   <button>Results</button>
 * </tabs-sk>
 * <tabs-panel-sk>
 *   <div>
 *     This is the query tab.
 *   </div>
 *   <div>
 *     This is the results tab.
 *   </div>
 * </tabs-panel-sk>
 *
 * @evt tab-selected-sk - Event sent when the user clicks on a tab. The events
 *        value of detail.index is the index of the selected tab.
 *
 */
class TabsSk extends HTMLElement {
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
    this.querySelectorAll('button').forEach((ele, i) => {
      if (ele === e.target) {
        ele.classList.add('selected');
        this._trigger(i, true);
      } else {
        ele.classList.remove('selected');
      }
    });
  }

  /**
   * Force the selection of a tab
   *
   * @param {number} index The index of the tab to select.
   * @param {boolean} [trigger=false] If true then trigger the 'tab-selected-sk' event.
   */
  select(index, trigger=false) {
    this.querySelectorAll('button').forEach((ele, i) => {
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
}

window.customElements.define('tabs-sk', TabsSk);

/**
 * <code>tabs-panel-sk</code>
 *
 * <p>
 *   See the description of [tabs-sk]{@link module:skia-elements/tabs-sk~TabsSk}.
 * </p>
 *
 * @attr selected - The index of the tab panel to display.
 *
 */
class TabsPanelSk extends HTMLElement {
  static get observedAttributes() {
    return ['selected'];
  }

  connectedCallback() {
    upgradeProperty(this, 'selected');
  }

  /** @prop {boolean} selected Mirrors the 'selected' attribute. */
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
}

window.customElements.define('tabs-panel-sk', TabsPanelSk);
