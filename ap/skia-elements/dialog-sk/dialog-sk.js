/** @module skia-elements/dialog-sk */

/** <code>dialog-sk</code>
 *
 * <p>
 *   A custom elment that creates a dialog centered in the window.
 *   Pressing the ESC key will cause the dialog to close.
 * </p>
 *
 * @example
 *
 * <dialog-sk id=dialog>
 *   <p>This is a dialog.</p>
 *   <button onclick="this.parentElement.shown = false;">Close</button>
 * </dialog-sk>
 *
 * @attr shown - A boolean attribute that is present when the dialog is shown.
 *            and absent when it is hidden.
 *
 * @evt closed - This event is generated when the dialog is closed.
 */
class DialogSk extends HTMLElement {
  static get observedAttributes() {
    return ['shown'];
  }

  /** @prop {boolean} shown Mirrors the shown attribute. */
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
}

window.customElements.define('dialog-sk', DialogSk);
