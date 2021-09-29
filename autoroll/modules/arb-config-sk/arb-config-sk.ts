/**
 * @module autoroll/modules/arb-config-sk
 * @description <h2><code>arb-config-sk</code></h2>
 *
 * <p>
 * This element provides UI for editing the configuration of a roller.
 * </p>
 */

import { html } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { define } from 'elements-sk/define';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/styles/table';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LoginTo } from '../../../infra-sk/modules/login';

import { Config } from '../config';

import {
  AutoRollService,
  GetAutoRollService,
} from '../rpc';

const inputID = 'configInput';

export class ARBConfigSk extends ElementSk {
  private static template = (ele: ARBConfigSk) => (!ele.config
    ? html``
    : html`
  <div>
    <button
      @click="${() => {
          $$<HTMLTextAreaElement>(`#${inputID}`, ele)!.value = ele.configJSON;
    }}"
      title="Revert to the checked-in config."
      >Revert</button>
    <button
      @click="${() => ele.submit()}"
      title="Update the roller config."
    >Submit</button>
  </div>
  <div>
    <textarea
      id="${inputID}"
      label="Edit the roller config."
      rows=${ele.configJSON.split('\n').length}
      cols=120
      >${ele.configJSON}</textarea>
  </div>
  <div style="display:none">
    <form id="configForm" action="/config" method="post">
      <textarea id="configJson" name="configJson"></textarea>
    </form>
  </form>
`);

  private config: Config = {} as Config;

  private configJSON: string = '';

  private rpc: AutoRollService = GetAutoRollService(this);

  constructor() {
    super(ARBConfigSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    const params = new URLSearchParams(window.location.search);
    const rollerName = params.get('roller');
    if (rollerName) {
      this.loadConfig(rollerName);
    }
    this._render();
  }

  private loadConfig(roller: string) {
    console.log(`Loading config for ${roller}...`);

    fetch(`/r/${roller}/config`).then(jsonOrThrow).then((cfg: Config) => {
      this.config = cfg;
      this.configJSON = JSON.stringify(this.config, null, 2);
      this._render();
    });
  }

  private submit() {
    // TODO(borenet): This is goofy because we have two textareas which both
    // contain the config in JSON format, but eventually we'll have UI for
    // editing the config.
    const configJSON = $$<HTMLTextAreaElement>(`#${inputID}`, this)!.value;
    const config = JSON.parse(configJSON) as Config;
    $$<HTMLTextAreaElement>('#configJson', this)!.value = configJSON;
    $$<HTMLFormElement>('#configForm', this)!.submit();
  }
}

define('arb-config-sk', ARBConfigSk);
