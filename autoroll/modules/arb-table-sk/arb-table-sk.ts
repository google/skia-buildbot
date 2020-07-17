/**
 * @module autoroll/modules/arb-table-sk
 * @description <h2><code>arb-table-sk</code></h2>
 *
 * <p>
 * This element displays the list of active Autorollers.
 * </p>
 */

import { html } from 'lit-html'

import { define } from 'elements-sk/define'
import 'elements-sk/styles/table';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { AutoRollMiniStatuses, AutoRollRPCsClient} from '../rpc/rpc';

export class ARBTableSk extends ElementSk {
  private static template = (ele: ARBTableSk) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${ele.rollers.statuses?.map((st) => html`
    <tr>
      <td>
        <a href="/r/${st.roller}">${st.childname} into ${st.parentname}</a>
      </td>
      <td>${st.mode}</td>
      <td>${st.numbehind}</td>
      <td>${st.numfailed}</td>
    </tr>
  `)}
  </table>
`;
  private rollers: AutoRollMiniStatuses = {statuses:[]};

  constructor() {
    super(ARBTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.reload();
  }

  fetch(input: RequestInfo, init?: RequestInit | undefined): Promise<Response> {
    console.log("fetching");
    console.log(input);
    return fetch(input, init);
  }

  private reload() {
    console.log("loading")
    const host = window.location.protocol + "//" + window.location.host;
    const rpcs = new AutoRollRPCsClient(host, this.fetch.bind(this));
    this.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
    rpcs.view_GetRollers({}).then((rollers) => {
      console.log("fetched:");
      console.log(rollers);
      this.rollers = rollers;
      this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      this._render();
    }, (err: any) => {
      console.log(err);
      this.dispatchEvent(new CustomEvent('fetch-error', {
        detail: {
          error: err,
          loading: "GetRollers",
        },
        bubbles: true,
      }));
    }).catch((err: any) => {
      console.log(err);
      this.dispatchEvent(new CustomEvent('fetch-error', {
        detail: {
          error: err,
          loading: "GetRollers",
        },
        bubbles: true,
      }));
    });
  }
}

define('arb-table-sk', ARBTableSk);