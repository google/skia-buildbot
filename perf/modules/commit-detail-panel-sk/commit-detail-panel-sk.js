/**
 * @module modules/commit-detail-panel-sk
 * @description <h2><code>commit-detail-panel-sk</code></h2>
 *
 * @evt commit-selected - Event produced when a commit is selected. The
 *     the event detail contains the serialized cid.CommitDetail and
 *     a simplified description of the commit:
 *
 *     <pre>
 *     {
 *       description: "foo (foo@example.org) 62W Commit from foo.",
 *       commit: {
 *         author: "foo (foo@example.org)",
 *         url: "skia.googlesource.com/bar",
 *         message: "Commit from foo.",
 *         ts: 1439649751,
 *       },
 *     }
 *     </pre>
 *
 * @attr {Boolean} selectable - A boolean attribute that if true means
 *     that the commits are selectable, and when selected
 *     the 'commit-selected' event is generated.
 *
 * @attr {Number} selected - The index of the selected commit.
 */
import { html, render } from 'lit-html'
import { findParent } from 'common-sk/modules/dom'
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import '../commit-detail-sk'

const rows = (ele) => ele.details.map((item, index) => html`
  <tr data-id="${index}" ?selected="${ele._isSelected(index)}">
    <td>${ele._trim(item.author)}</td>
    <td>
      <commit-detail-sk .cid=${item}></commit-detail-sk>
    </td>
  </tr>
`);

const template = (ele) => html`
  <table @click=${ele._click}>
    ${rows(ele)}
  </table>
`;

window.customElements.define('commit-detail-panel-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this._details = [];
    upgradeProperty(this, 'details');
    upgradeProperty(this, 'selected');
    upgradeProperty(this, 'selectable');
    this._render();
  }

  /** @prop details {Array} An array of serialized cid.CommitDetail, e.g.
   *
   *  [
   *     {
   *       author: "foo (foo@example.org)",
   *       url: "skia.googlesource.com/bar",
   *       message: "Commit from foo.",
   *       ts: 1439649751,
   *     },
   *     ...
   *  ]
   */
  get details() { return this._details }
  set details(val) {
    this._details = val;
    this._render();
  }

  _isSelected(index) {
    return this.selectable && (index == this.selected );
  }

  _click(e) {
    let ele = findParent(e.target, 'TR');
    if (!ele) {
      return
    }
    this.selected = +ele.dataset['id']
    let detail = {
      description: ele.textContent.trim(),
      commit: this._details[this.selected],
    }
    this.dispatchEvent(new CustomEvent('commit-selected', {detail: detail, bubbles: true}));
  }

  _trim(s) {
    s = s.slice(0, 72);
    return s;
  }

  static get observedAttributes() {
    return ['selectable', 'selected'];
  }

  /** @prop selectable {string} Mirrors the selectable attribute. */
  get selectable() { return this.hasAttribute('selectable'); }
  set selectable(val) {
    if (val) {
      this.setAttribute('selectable', '');
    } else {
      this.removeAttribute('selectable');
    }
  }

  /** @prop selected {Number} Mirrors the selected attribute. */
  get selected() {
    if (this.hasAttribute('selected')) {
      return +this.getAttribute('selected')
    } else {
      return -1;
    }
  }
  set selected(val) {
    this.setAttribute('selected', ''+val);
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue !== newValue) {
      this._render();
    }
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }
});
