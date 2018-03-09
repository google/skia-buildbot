/**
 * @module skia-elements/tabs-panel-sk
 * @description <h2><code>tabs-panel-sk</code></h2>
 *
 * <p>
 *   See the description of [tabs-sk]{@link module:skia-elements/tabs-sk}.
 * </p>
 *
 * @attr selected - The index of the tab panel to display.
 *
 */
import { upgradeProperty } from '../upgradeProperty';

window.customElements.define('tabs-panel-sk', class extends HTMLElement {
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
});
