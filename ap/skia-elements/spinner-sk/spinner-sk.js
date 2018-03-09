/** @module skia-elements/spinner-sk */
import { upgradeProperty } from '../upgradeProperty'

/**
 * <code>spinner-sk</code>
 * <p>
 *   An activity spinner.
 * </p>
 *
 * @attr active - Boolean attribute, if present, spinner is active.
 *
 */
class SpinnerSk extends HTMLElement {
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
}

window.customElements.define('spinner-sk', SpinnerSk);
