//  dialog-sk is a custom elment that creates a dialog centered in the window.
//
//  Properties:
//    shown - True if the dialog is showing, false if it is not. Reflected
//            to/from the shown attribute.
//
//  Attributes:
//    shown - A boolean attribute that is present when the dialog is shown.
//            and absent when it is hidden.
//
//  Methods:
//    None.
//
//  Events:
//    closed - This event is generated when the dialog is closed.
//
window.customElements.define('dialog-sk', class extends HTMLElement {
  static get observedAttributes() {
    return ['shown'];
  }

  get shown() { return this.hasAttribute('shown'); }
  set shown(val) {
    if (val) {
      this.setAttribute('shown', '');
    } else {
      this.removeAttribute('shown');
    }
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (newValue !== null) {
      window.addEventListener('keydown', this);
    } else {
      window.removeEventListener('keydown', this);
      this.dispatchEvent(new CustomEvent('closed', { bubbles: true }));
    }
  }

  handleEvent(e) {
    if (e.key === "Escape") {
      e.preventDefault();
      this.shown = false;
    }
  }
});
