import { $$ } from '../dom'

class RadioGroupElement extends HTMLElement {
  constructor() {
    super();
    this._obs = new MutationObserver(() => this._rationalize());
  }

  connectedCallback() {
    if (!this.hasAttribute('role')) {
      this.setAttribute('role', 'radio');
    }
    if (!this.hasAttribute('tabindex')) {
      this.setAttribute('tabindex', 0);
    }
    this.addEventListener('selected-sk', this);
    this.addEventListener('keydown', this);
    this._obs.observe(this, {
      childList: true,
      subtree: true,
    });
    this._rationalize();
  }

  disconnectedCallback() {
    this.removeEventListener('selected-sk', this);
    this.removeEventListener('keydown', this);
    this._obs.disconnect();
  }

  _prev(all) {
    let p = all[all.length-1];
    for (let i=0; i<all.length; i++) {
      let ele = all[i];
      if (ele.checked) {
        return p;
      } else {
        p = ele;
      }
    }
    return undefined;
  }

  _next(all) {
    let found = false;
    for (let i=0; i<all.length; i++) {
      let ele = all[i];
      if (found) {
        return ele;
      } else if (ele.checked) {
        found = true;
      }
    }
    return all[0];
  }

  handleEvent(e) {
    if (e.type === 'keydown') {
      let all = $$('[role="radio"]', this);
      if (event.altKey) {
        return;
      }
      if (e.keyCode >= 35 && e.keyCode <= 40) {
          e.preventDefault();
      }
      switch (e.keyCode) {
        case 37: // Left
        case 38: // Up
          this._clearExcept(all, this._prev(all));
          break;

        case 39: // Right
        case 40: // Down
          this._clearExcept(all, this._next(all));
          break;

        case 36: // Home
          this._setChecked(all[0]);
          break;

        case 35: // End
          this._clearExcept(all, all[all.length-1]);
          break;

        default:
          break;
      }
    } else if (e.target.getAttribute('role') === 'radio') {
      let all = $$('[role="radio"]', this);
      this._clearExcept(all, e.target.checked ? e.target : undefined);
    }
  }

  // _rationalize fixes up the tabindex so that focus moves correctly, i.e. if
  // the radio-group-sk has no children then the focus lands on the
  // radio-group-sk itself, otherwise the focus should move to the selected
  // child on tab.
  _rationalize() {
    let all = $$('[role="radio"]', this);
    if (!all.length) {
      this.setAttribute('tabindex', 0);
      return
    }
    this.setAttribute('tabindex', -1);
    let checked = $$('[role="radio"][aria-checked="true"]', this);
    this._clearExcept(all, checked[0]);
    if (!checked.length) {
      all[0].setAttribute('tabindex', 0);
    }
  }

  _clearExcept(all, checked) {
    all.forEach((ele) => {
      if (ele === checked) {
        ele.checked = true;
        ele.focus();
        ele.setAttribute('tabindex', 0);
      } else {
        ele.checked = false;
        ele.setAttribute('tabindex', -1);
      }
    });
    if (!checked && all.length > 0) {
      all[0].setAttribute('tabindex', 0);
    }
  }
}

window.customElements.define('radio-group-sk', RadioGroupElement);
