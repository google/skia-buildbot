/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */

import { html } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';

import { define } from 'elements-sk/define';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/styles/table';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LoginTo } from '../../../infra-sk/modules/login';

import {
  AutoRollService,
  Config,
  GetAutoRollService,
  GetConfigResponse,
  PutConfigResponse,
} from '../rpc';

export class ARBConfigSk extends ElementSk {
  private static template = (ele: ARBConfigSk) =>
    !ele.config
      ? html``
      : html`
  <div>
    <button
      @click="${() => {
          $$<HTMLTextAreaElement>('#configInput', ele)!.value = ele.configJSON;
        }}"
      title="Revert to the checked-in config."
      >Revert</button>
    <button
      @click="${() => ele.submit()}"
      ?disabled="${!ele.editRights}"
      title="${ele.editRights
          ? 'Update the roller config.'
          : 'Please log in to make changes.'}"
    >Submit</button>
  </div>
  <div>
    <textarea
      id="configInput"
      label="Edit the roller config."
      rows=${ele.configJSON.split('\n').length}
      cols=120
      >${ele.configJSON}</textarea>
  </div>
`;

  private config: Config | null = null;
  private configJSON: string = "";
  private editRights: boolean = false;
  private rpc: AutoRollService = GetAutoRollService(this);

  constructor() {
    super(ARBConfigSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('roller');
    this._render();
    LoginTo('/loginstatus/').then((loginstatus: any) => {
      this.editRights = loginstatus.IsAGoogler;
      this._render();
    });
    this.reload();
  }

  get roller() {
    return this.getAttribute('roller') || '';
  }
  set roller(v: string) {
    this.setAttribute('roller', v);
    this.reload();
  }

  private reload() {
    if (!this.roller) {
      return;
    }
    console.log('Loading config for ' + this.roller + '...');
    this.rpc
      .getConfig({ rollerId: this.roller })
      .then((resp: GetConfigResponse) => {
        this.config = resp.config!;
        this.configJSON = JSON.stringify(this.config, null, 2);
        this._render();
      });
  }

  private submit() {
    const configJSON = $$<HTMLTextAreaElement>('#configInput', this)!.value;
    const config = JSON.parse(configJSON) as Config;
    this.rpc.putConfig({
      config: config,
      commitMsg: 'Update config for ' + this.roller, // TODO(borenet): Prompt for commit message.
    }).then((resp: PutConfigResponse) => {
      // TODO(borenet): Add a popup, replace the textarea with a link, or just
      // redirect to the CL itself.
      alert(resp.cl);
    })
  }
}

define('arb-config-sk', ARBConfigSk);
