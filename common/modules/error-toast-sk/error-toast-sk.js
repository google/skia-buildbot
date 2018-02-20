import 'skia-elements/toast-sk'
import { upgradeProperty } from 'skia-elements/upgradeProperty'

//  Listens for 'error-sk' events that bubble up to the document and displays them.
//  The 'error-sk' event should have 'detail' of the form:
//    {
//      message: "The error message to display goes here.",
//      duration: Integer, the number of ms to display or 0 for indefinitely.
//                Defaults to 10000 (10s)
//    }
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    None
window.customElements.define('error-toast-sk', class extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `<toast-sk></toast-sk>`;
    this._toast = this.firstElementChild;
    document.addEventListener('error-sk', this);
  }

  disconnectedCallback() {
    document.removeEventListener('error-sk', this);
  }

  handleEvent(e) {
    if (e.detail.duration) {
      this._toast.duration = e.detail.duration
    }
    this._toast.textContent = e.detail.message;
    this._toast.show();
  }
});
