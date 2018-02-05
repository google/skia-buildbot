import { upgradeProperty } from 'skia-elements/upgrade-property'

// select-sk is a selection element that allows any kind of children.
//
// Properties:
//   selection - The index of the item selected. Has a value of -1 if nothing
//               is selected.
//
// Attributes:
//   None
//
// Methods:
//   None
//
// Events:
//   selection-changed - Sent when an item is clicked and the selection is changed.
//                       The detail of the event contains the child element index:
//
//        detail: {
//          selection: 1,
//        }
//
window.customElements.define('select-sk', class extends HTMLElement {
  constructor() {
    super();
    // Keep _selection up to date by monitoring DOM changes.
    this._obs = new MutationObserver(() => this._bubbleUp());
    this._selection = -1;
  }

  connectedCallback() {
    upgradeProperty(this, 'selection');
    this.addEventListener('click', this._click);
    this._obs.observe(this, {
      childList: true,
      attributes: true,
      attributeFilter: ["selected"],
    });
    this._bubbleUp();
  }

  disconnectedCallback() {
    this.removeEventListener('click', this._click);
    this._obs.disconnect();
  }

  get selection() { return this._selection; }
  set selection(val) {
    this._selection = val;
    this._rationalize();
  }

  _click(e) {
    let oldIndex = this._selection;
    // Look up the DOM path until we find an element that is a child of
    // 'this', and set _selection based on that.
    e.path.forEach(ele => {
      if (ele.parentElement === this) {
        for (let i = 0; i < this.children.length; i++) {
          if (this.children[i] === ele) {
            this._selection = i;
          }
        }
      }
    });
    this._rationalize();
    if (oldIndex != this._selection) {
      this.dispatchEvent(new CustomEvent('selection-changed', {
        detail: {
          selection: this._selection,
        },
        bubbles: true,
      }));
    }
  }

  // Loop over all immediate child elements and make sure at most only one is selected.
  _rationalize() {
    for (let i = 0; i < this.children.length; i++) {
      if (this._selection === i) {
        this.children[i].setAttribute('selected', '');
      } else {
        this.children[i].removeAttribute('selected');
      }
    }
  }

  // Loop over all immediate child elements and find the first one selected.
  _bubbleUp() {
    for (let i = 0; i < this.children.length; i++) {
      if (this.children[i].hasAttribute('selected')) {
        this._selection = i;
        break;
      }
    }
    this._rationalize();
  }
});
