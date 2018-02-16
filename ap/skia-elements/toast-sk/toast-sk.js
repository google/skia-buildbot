import { upgradeProperty } from '../upgradeProperty'

// The <toast-sk> custom element declaration.
//
//   Notification toast that pops up from the bottom of the screen
//   when shown.
//
//  Attributes:
//    duration - The duration, in ms, to display the notification.
//               Defaults to 5000. A value of 0 means to display
//               forever.
//
//  Properties:
//    duration - Mirrors the 'duration' attribute.
//
//  Events:
//    None
//
//  Methods:
//    show() - Displays the contents of the toast.
//
//    hide() - Hides the contents of the toast.
//
window.customElements.define('toast-sk', class extends HTMLElement {
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

  get duration() { return +this.getAttribute('duration'); }
  set duration(val) { this.setAttribute('duration', val); }

  show() {
    this.setAttribute('shown', '');
    if (this.duration > 0 && !this._timer) {
      this._timer = window.setTimeout(() => {
        this._timer = null;
        this.hide();
      }, this.duration);
    }
  }

  hide() {
    this.removeAttribute('shown');
    if (this._timer) {
      window.clearTimeout(this._timer);
      this._timer = null;
    }
  }
});
