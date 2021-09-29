import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';

import 'elements-sk/error-toast-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { upgradeProperty } from 'elements-sk/upgradeProperty';

import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import '../../../infra-sk/modules/systemd-unit-status-sk';

const listUnits = (ele) => ele._units.map(
  (unit) => html`<systemd-unit-status-sk machine="${ele._hostname}" .value=${unit}></systemd-unit-status-sk>`,
);

const template = (ele) => html`
<header>
  <h1>pulld - ${ele._hostname}</h1>
</header>
<main>
  <div @unit-action=${ele._unitAction}>
    ${listUnits(ele)}
  </div>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

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
define('pulld-app-sk', class extends HTMLElement {
  constructor() {
    super();

    // The systemd units, an array of systemd.UnitStatus.
    this._units = [];

    // The hostname of the server.
    this._hostname = '';
  }

  connectedCallback() {
    this._loadData();
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }

  _loadData() {
    fetch('/_/list', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._units = json.units;
      this._hostname = json.hostname;
      this._render();
    }).catch(errorMessage);
    this._render();
  }

  _unitAction(e) {
    const params = {
      name: e.detail.name,
      action: e.detail.action,
    };
    fetch(`/_/change?${fromObject(params)}`, {
      credentials: 'include',
      method: 'POST',
      headers: new Headers({
        'Content-Type': 'application/json',
      }),
    }).then(jsonOrThrow).then((json) => {
      errorMessage(`${e.detail.name}: ${json.result}`);
      this._loadData();
    }).catch(errorMessage);
  }
});
