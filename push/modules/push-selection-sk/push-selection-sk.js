import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';

import 'elements-sk/icon/check-icon-sk';
import 'elements-sk/icon/warning-icon-sk';
import 'elements-sk/select-sk';
import 'elements-sk/styles/buttons';
import { upgradeProperty } from 'elements-sk/upgradeProperty';

import { diffDate } from 'common-sk/modules/human';

function linkToCommit(hash) {
  return `https://skia.googlesource.com/buildbot/+show/${hash}`;
}

function shorten(s) {
  return s.slice(0, 8);
}

const dirtyIndicator = (choice) => (choice.Dirty ? html`<warning-icon-sk title="Uncommited changes when the package was built."></warning-icon-sk>` : '');

const listChoices = (choices) => choices.map((choice) => html`
  <div class=pushSelection data-name="${choice.Name}">
    <check-icon-sk title="Currently installed."></check-icon-sk>
    <pre><a target=_blank href="${linkToCommit(choice.Hash)}">${shorten(choice.Hash)}</a></pre>
    <pre class=built>${diffDate(choice.Built)}</pre>
    <pre class=userid title="${choice.UserID}">${shorten(choice.UserID)}</pre>
    <span>${choice.Note}</span>
    ${dirtyIndicator(choice)}
  </div>`);

const template = (ele) => html`
  <dialog>
    <h2>Choose a release package to push</h2>
    <select-sk selection="${ele._chosen}" @selection-changed=${ele._selectionChanged}>
      ${listChoices(ele._choices)}
    </select-sk>
    <div class=buttons>
      <button @click=${ele.hide}>Cancel</button>
    </div>
  </dialog>`;

/** <code>push-selection-sk</code> custom element declaration.
 *
 * <p>
 *  Presents a dialog of package choices and generates an event when the user has
 *  made a selection. It is a custom element used by <push-server-sk>.
 * </p>
 *
 * @evt package-change A 'package-change' event is generated when the user
 *   selects a package to push. The event detail has the following shape:
 *
 * <pre>
 * {
 *   name: 'package name goes here', // The full name of the package selected.
 * }
 * </pre>
 */
class PushSelectionSk extends HTMLElement {
  constructor() {
    super();
    this._choices = [];
    this._chosen = -1;
  }

  connectedCallback() {
    upgradeProperty(this, 'choices');
    upgradeProperty(this, 'chosen');
    this._render();
    this._dialog = this.firstElementChild;
    dialogPolyfill.registerDialog(this._dialog);
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }

  /** @prop {Array} The list of packages that are available. Serialized Package from infra/go/packages/. For example:
   *
   * <pre>
   *   {
   *     Name: 'pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-08T02:09:58Z:79f6b17ea316c5d877f4f1e3fa9c7a4ea950916c.deb',
   *     Hash: '79f6b17ea316c5d877f4f1e3fa9c7a4ea950916c',
   *     UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
   *     Built: '2014-12-08T02:09:58Z',
   *     Dirty: true,
   *     Note: 'some reason for a push'
   *   },
   * </pre>
   */
  get choices() { return this._choices; }

  set choices(val) {
    this._choices = val;
    this._render();
  }

  /** @prop {number} The index of the chosen package. */
  get chosen() { return this._chosen; }

  set chosen(val) {
    this._chosen = +val;
    this._render();
  }

  _selectionChanged(e) {
    this._chosen = e.detail.selection;
    this.dispatchEvent(new CustomEvent('package-change', {
      detail: {
        name: this.choices[e.detail.selection].Name,
      },
      bubbles: true,
    }));
  }

  /** Show the dialog. */
  show() {
    this._dialog.showModal();
  }

  /** Hide the dialog. */
  hide() {
    this._dialog.close();
  }
}

define('push-selection-sk', PushSelectionSk);
