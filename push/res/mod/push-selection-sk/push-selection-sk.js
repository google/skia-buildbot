import 'skia-elements/buttons'
import 'skia-elements/dialog-sk'
import 'skia-elements/icon-sk'
import { upgradeProperty } from 'skia-elements/dom'

import 'common/select-sk'
import { diffDate } from 'common/human'

import { html, render } from 'lit-html/lib/lit-extended'

function linkToCommit(hash) {
  return 'https://skia.googlesource.com/buildbot/+/' + hash;
}

function shorten(s) {
  return s.slice(0, 8);
}

function dirtyIndicator(choice) {
    return choice.Dirty ? html`<icon-warning-sk title="Uncommited changes when the package was built."></icon-warning-sk>` : ``;
}

function listChoices(choices) {
  return choices.map((choice) => html`
<div class=pushSelection data-name$="${choice.Name}">
  <icon-check-sk title="Currently installed."></icon-check-sk>
  <pre><a target=_blank href="${linkToCommit(choice.Hash)}">${shorten(choice.Hash)}</a></pre>
  <pre class=built>${diffDate(choice.Built)}</pre>
  <pre class=userid title$="${choice.UserID}">${shorten(choice.UserID)}</pre>
  <span>${choice.Note}</span>
  ${dirtyIndicator(choice)}
</div>`);
}

const template = (ele) => html`
<dialog-sk>
  <h2>Choose a release package to push</h2>
  <select-sk selection="${ele._chosen}" on-selection-changed=${e => ele._selectionChanged(e)}>
    ${listChoices(ele._choices)}
  </select-sk>
  <div class=buttons>
    <button on-click=${() => ele.hide()}>Cancel</button>
  </div>
</dialog-sk>`;

// The <push-selection-sk> custom element declaration.
//
//  Presents a dialog of package choices and generates an event when the user has
//  made a selection. It is a custom element used by <push-server-sk>.
//
//  Attributes:
//    None
//
//  Properties:
//    'choices'
//        The list of packages that are available. Serialized Package from infra/go/packages/. For example:
//
//          {
//            Name: 'pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-08T02:09:58Z:79f6b17ea316c5d877f4f1e3fa9c7a4ea950916c.deb',
//            Hash: '79f6b17ea316c5d877f4f1e3fa9c7a4ea950916c',
//            UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
//            Built: '2014-12-08T02:09:58Z',
//            Dirty: true,
//            Note: 'some reason for a push'
//          },
//
//    'chosen'
//        Number - The index of the chosen package.
//
//  Events:
//    'package-change'
//        A 'package-change' event is generated when the user selects a package to push.
//        The change event has the following attributes:
//
//          event.detail.name   - The full name of the package selected.
//
//  Methods:
//    show() - Shows the dialog.
//    hide() - Hides the dialog.
window.customElements.define('push-selection-sk', class extends HTMLElement {
  constructor() {
    super()
    this._choices = [];
    this._chosen = -1;
  }

  connectedCallback() {
    upgradeProperty(this, 'choices');
    upgradeProperty(this, 'chosen');
    this._render();
  }

  _render() {
    render(template(this), this);
  }

  get choices() { return this._choices; }
  set choices(val) {
    this._choices = val;
    this._render();
  }

  get chosen() { return this._chosen; }
  set chosen(val) {
    this._chosen = +val;
    this._render();
  }

  _selectionChanged(e) {
    this.dispatchEvent(new CustomEvent('package-change', {
      detail: {
        name: this.choices[e.detail.selection].Name,
      },
      bubbles: true,
    }));
  }

  show() {
    this.firstElementChild.show();
  }

  hide() {
    this.firstElementChild.hide();
  }

});
