import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/radio-sk';
import 'elements-sk/styles/select';
import 'elements-sk/icon/find-in-page-icon-sk';

const patchSet = (ps, ele) => html`
<option ?selected=${ele.ps_order === ps.order}>PS ${ps.order}</option>
`;

const tryJob = (tj) => html`
<div class=tryjob title=${tj.name}>
  <a href=${tj.url} target=_blank rel=noopener>
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
<div class=info>
  <span class=title>${cl.system} changelist:</span>
  <a href=${cl.url} target=_blank rel=noopener>
    ${limitString(cl.subject, 48)}
  </a>

  <span>${limitString(cl.owner, 32)}</span>

  <a href="/triagelog?changelist_id=${cl.id}&crs=${cl.system}">
    <find-in-page-icon-sk></find-in-page-icon-sk>Triagelog
  </a>
</div>

<div class=inputs>
  <select @input=${ele._onSelectPS}>
    ${ele._summary.patch_sets.map((ps) => patchSet(ps, ele))}
  </select>
  <span class=spacer></span>
  <div class=radiogroup>
    <radio-sk label="exclude results from master" name=include_master
      ?checked=${!ele.include_master} @change=${() => ele._masterChange(false)}></radio-sk>
    <radio-sk label="show all results" name=include_master
       ?checked=${ele.include_master} @change=${() => ele._masterChange(true)}></radio-sk>
  </div>
</div>

<div class=tryjob-container>
  ${ps.try_jobs.map((tj) => tryJob(tj))}
</div>
`;
};

function limitString(s, maxLength) {
  if (s.length <= maxLength) {
    return s;
  }
  return `${s.substring(0, maxLength - 3)}...`;
}

define('changelist-controls-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._psOrder = 0; // default to use the last PatchSet.
    this._includeMaser = false;
    this._summary = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop ps_order {int} the order of the PatchSet currently being shown. */
  get ps_order() { return this._psOrder; }

  set ps_order(val) {
    this._psOrder = +val;
    this._render();
  }

  /** @prop include_master {bool} if we should show results that are also
   *    on master, as opposed to those that are exclusive . */
  get include_master() { return this._includeMaser; }

  set include_master(val) {
    this._includeMaser = val !== 'false' && !!val;
    this._render();
  }

  _masterChange(newVal) {
    this.include_master = newVal; // calls _render()
    this._sendUpdateEvent();
  }

  _onSelectPS(e) {
    const xps = this._summary.patch_sets;
    const ps = xps[e.target.selectedIndex];
    this.ps_order = ps.order; // calls _render()
    this._sendUpdateEvent();
  }

  /* _selectedPS returns the PatchSet object which matches _psOrder. if _psOrder is 0
     (match latest), _psOrder will be updated to whatever the latest order is.
   */
  _selectedPS() {
    if (!this._summary || !this._summary.patch_sets || !this._summary.patch_sets.length) {
      return null;
    }
    const xps = this._summary.patch_sets;
    if (!this._psOrder) {
      const o = xps[xps.length - 1];
      this._psOrder = o.order;
      return o;
    }
    for (let i = 0; i < xps.length; i++) {
      if (xps[i].order === this._psOrder) {
        return xps[i];
      }
    }
    return null;
  }

  _sendUpdateEvent() {
    this.dispatchEvent(new CustomEvent('cl-control-change', {
      detail: {
        include_master: this.include_master,
        ps_order: this.ps_order,
      },
      bubbles: true,
    }));
  }

  /** setSummary sets the frontend.ChangeListSummary for this element to display. */
  setSummary(sum) {
    this._summary = sum;
    this._render();
  }
});
