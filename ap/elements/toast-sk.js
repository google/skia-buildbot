import { upgradeProperty } from './upgrade-property.js'

window.customElements.define('toast-sk', class extends HTMLElement {
  constructor() {
    super();
    this._timer = undefined;
  }

  connectedCallback() {
    if (!this.hasAttribute('duration')) {
      this.duration = 5000;
    }
    upgradeProperty(this, 'duration');
  }

  get duration() { return +this.getAttribute('duration'); }
  set duration(val) { this.setAttribute('duration', val); }

  show() {
    this.setAttribute('shown', '');
    if (this.duration > 0 && this._timer === undefined) {
      this._timer = window.setTimeout(() => {
        this._timer = undefined;
        this.hide();
      }, this.duration);
    }
  }

  hide() {
    this.removeAttribute('shown');
    if (this._timer !== undefined) {
      window.clearTimeout(this._timer);
      this._timer = undefined;
    }
  }
});
