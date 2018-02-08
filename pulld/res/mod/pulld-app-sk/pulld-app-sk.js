import { upgradeProperty } from 'skia-elements/upgrade-property'
import { html, render } from 'lit-html/lib/lit-extended'

import 'common/error-toast-sk'
import 'common/systemd-unit-status-sk'
import { errorMessage } from 'common/errorMessage'
import { fromObject } from 'common/query'

function listUnits(ele) {
  return ele._units.map(unit => {
    return html`
      <systemd-unit-status-sk machine$="${ele._hostname}" value=${unit}></systemd-unit-status-sk>
    `;
  });
}

const template = (ele) => html`
<header>
  <h1>pulld - ${ele._hostname}</h1>
</header>
<main>
  <div on-unit-action=${e => ele._unitAction(e)}>
    ${listUnits(ele)}
  </div>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

const jsonOrThrow = (resp) => {
  if (resp.ok) {
    return resp.json();
  }
  throw 'Bad network response.';
}

// The <pulld-app-sk> custom element declaration.
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
//
window.customElements.define('pulld-app-sk', class extends HTMLElement {
  constructor() {
    super();
    this._units = [];
    this._hostname = '';
  }

  connectedCallback() {
    this._loadData();
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

  _loadData() {
    fetch('/_/list').then(jsonOrThrow).then(json => {
      this._units = json.units;
      this._hostname = json.hostname;
      this._render();
    }).catch(errorMessage);
    this._render();
  }

  _unitAction(e) {
    let params = {
      name: e.detail.name,
      action: e.detail.action,
    }
    fetch('/_/change?' + fromObject(params), {
      method: 'POST',
      headers: new Headers({
        'Content-Type': 'application/json'
      })
    }).then(jsonOrThrow).then(json => {
      errorMessage(e.detail.name + ": " + json.result);
      this._loadData();
    }).catch(errorMessage);
  }

});
