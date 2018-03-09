/**
 * @module skia-elements/collapse-sk
 * @description <h2><code>collapse-sk</code></h2>
 *
 * <p>
 *   Is a collapsable element, upon collapse the element and its children
 *   are no longer displayed.
 * </p>
 *
 *  @attr closed - A boolean attribute that, if present, causes the element to
 *     collapse, i.e., transition to display: none.
 *
 */
import { upgradeProperty } from '../upgradeProperty'

window.customElements.define('collapse-sk', class extends HTMLElement {
  connectedCallback() {
    upgradeProperty(this, 'closed');
  }

  /** @prop {boolean} closed Mirrors the closed attribute. */
  get closed() { return this.hasAttribute('closed'); }
  set closed(val) {
    if (val) {
      this.setAttribute('closed', '');
    } else {
      this.removeAttribute('closed');
    }
  }
});
