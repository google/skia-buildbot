import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'

import 'elements-sk/radio-sk'
import 'elements-sk/styles/select'
import 'elements-sk/icon/find-in-page-icon-sk'

const patchSet = (ps, ele) => html`
<option ?selected=${ele._psOrder === ps.order}>PS ${ps.order}</option>
`;

const tryJob = (tj) => html`
<div class=tryJob>
  <a href=${tj.url} target="_blank" rel="noopener">
    ${limitString(tj.name, 60)}
  </a>
</div>
`;

const template = (ele) => {
  if (!ele._summary) {
    return '';
  }
  const cl = ele._summary.cl;
  const ps = ele._selectedPS();
  return html`
<div>
  <span>${cl.system} issue:</span>
  <a href=${cl.url} target="_blank" rel="noopener">
    ${limitString(cl.subject, 48)}
  </a>

  <span>${limitString(cl.owner, 32)}</span>

  <a href="/triagelog?issue=${cl.id}&system=${cl.system}">
    <find-in-page-icon-sk></find-in-page-icon-sk>Triagelog
  </a>
</div>

<div>
  <select @input=${ele._onSelectPS}>
    ${ele._summary.patch_sets.map((ps) => patchSet(ps, ele))}
  </select>

  <radiogroup>
    <radio-sk label="exclude results from master"></radio-sk>
    <radio-sk label="show all results"></radio-sk>
  </radiogroup>
</div>

<div>
  ${ps.try_jobs.map((tj) => tryJob(tj))}
</div>
`;
}

function limitString(s, maxLength) {
  if (s.length <= maxLength) {
    return s;
  }
  return s.substring(0, maxLength-3) + "...";
}

const USE_LAST = -1;

define('changelist-controls-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._psOrder = USE_LAST;
    this._summary = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _onSelectPS(e) {
    const xps = this._summary.patch_sets;
    const ps = xps[e.target.selectedIndex];
    this._psOrder = ps.order;
    this._render();
    // TODO(kjlubick): dispatch an event
  }

  _selectedPS() {
    if (!this._summary || !this._summary.patch_sets || !this._summary.patch_sets.length) {
      return null;
    }
    const xps = this._summary.patch_sets;
    if (this._psOrder === USE_LAST) {
      const o = xps[xps.length-1]
      this._psOrder = o.order;
      return o;
    }
    for(let i = 0; i < xps.length; i++){
      if (xps[i].order === this._psOrder) {
        return xps[i];
      }
    }
    return null;
  }

  setSummary(sum) {
    this._summary = sum;
    this._render()
  }

});