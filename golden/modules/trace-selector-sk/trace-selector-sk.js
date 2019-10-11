import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'

import 'elements-sk/icon/cancel-icon-sk'
import 'elements-sk/icon/check-circle-icon-sk'
import 'elements-sk/icon/help-icon-sk'
import 'elements-sk/icon/open-in-new-icon-sk'
import 'elements-sk/icon/radio-button-unchecked-icon-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/checkbox-sk'

const template = (ele) => html`
<div class=wrapper>

  <div class=title>
    Current query matches 18 traces
  </div>

  <table>
    <thead>
      <tr>
        <th></th>
        <th>Key</th>
        <th>Selected Values</th>
        <th>Current Digest</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td><checkbox-sk></checkbox-sk></td>
        <td>alpha_type</td>
        <td><input value="Premul"></input></td>
        <td>Premul</td>
      </tr>
      <tr>
        <td><checkbox-sk checked></checkbox-sk></td>
        <td>arch</td>
        <td><input value="arm arm64"></input></td>
        <td>arm</td>
      </tr>
      <tr>
        <td><checkbox-sk checked></checkbox-sk></td>
        <td>config</td>
        <td><input value="glesmsaa4"></input></td>
        <td>gles glesdft glesmsaa4</td>
      </tr>
      <tr>
        <td><checkbox-sk checked></checkbox-sk></td>
        <td>name</td>
        <td><input value="my_test"></input></td>
        <td>my_test</td>
      </tr>
    </tbody>
  </table>

  <button>Confirm Query</button>
</div>
`;


define('trace-selector-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

});