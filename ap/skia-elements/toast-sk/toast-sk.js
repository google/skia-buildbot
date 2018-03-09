/** @module skia-elements/toast-sk */
import { upgradeProperty } from '../upgradeProperty'

/**
 * <code>toast-sk</code>
 *
 * <p>
 *   Notification toast that pops up from the bottom of the screen
 *   when shown.
 * </p>
 *
 * @attr duration - The duration, in ms, to display the notification.
 *               Defaults to 5000. A value of 0 means to display
 *               forever.
 */
class ToastSk extends HTMLElement {
  constructor() {
    super();
    this._timer = null;
  }

  connectedCallback() {
    if (!this.hasAttribute('duration')) {
      this.duration = 5000;
    }
    upgradeProperty(this, 'duration');
  }

  /** @prop {number} duration Mirrors the duration attribute. */
  get duration() { return +this.getAttribute('duration'); }
  set duration(val) { this.setAttribute('duration', val); }

  /** Displays the contents of the toast. */
  show() {
    this.setAttribute('shown', '');
    if (this.duration > 0 && !this._timer) {
      this._timer = window.setTimeout(() => {
        this._timer = null;
        this.hide();
      }, this.duration);
    }
  }

  /** Hides the contents of the toast. */
  hide() {
    this.removeAttribute('shown');
    if (this._timer) {
      window.clearTimeout(this._timer);
      this._timer = null;
    }
  }
}

window.customElements.define('toast-sk', ToastSk);
