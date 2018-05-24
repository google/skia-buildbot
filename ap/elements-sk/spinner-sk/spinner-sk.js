/**
 * @module skia-elements/spinner-sk
 * @decription <h2><code>spinner-sk</code></h2>
 *
 * <p>
 *   An activity spinner.
 * </p>
 *
 * @attr active - Boolean attribute, if present, spinner is active.
 *
 */
import { upgradeProperty } from '../upgradeProperty'

window.customElements.define('spinner-sk', class extends HTMLElement {
  // TODO(jcgregorio) What is ARIA for a spinner?
  connectedCallback() {
    upgradeProperty(this, 'active');
  }

  /** @prop {boolean} active Mirrors the attribute 'active'. */
  get active() { return this.hasAttribute('active'); }
  set active(val) {
    if (val) {
      this.setAttribute('active', '');
    } else {
      this.removeAttribute('active');
    }
  }
});
