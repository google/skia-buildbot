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
<div class="vertical_layout wrapper">
  <div class="horizontal_layout">
    <span class=bold>
      Test: my_test
    </span>

    <span class=spacer></span>

    <a href="#cluster">
      <radio-button-unchecked-icon-sk></radio-button-unchecked-icon-sk>
    </a>
  </div>

  <div class="horizontal_layout digests">
    <span class=bold>
      Left: 2135844182be4192208c96150065ddc3
    </span>
    <span class="limited spacer"></span>
    <span class=bold>
      Right: abcdef01234e4192208c96150065deff
    </span>
  </div>

  <!-- digests side by side-->
  <div class=horizontal_layout>
    <div class="vertical_layout details">
      <div>
        <a href="#details">Diff Details</a>
      </div>
      <div>Diff Metric: 2.34</div>
      <div>Diff %: 6.33</div>
      <div>Pixels: 19931</div>
      <div>Max RGBA: [255, 255, 255, 255]</div>

      <div class="triage vertical_layout">
        <div>
          <button class="positive"><check-circle-icon-sk></check-circle-icon-sk></button>
          <button class="negative"><cancel-icon-sk></cancel-icon-sk></button>
          <button class="untriaged selected"><help-icon-sk></help-icon-sk></button>
        </div>

        <textarea placeholder="Type a comment here"></textarea>
        <checkbox-sk label="apply to traces matching a query"></checkbox-sk>
        <button>Add comment</button>
      </div>
    </div>

    <div class=vertical_layout>
      <div class=horizontal_layout>
        <div class=preview>
          <img src="https://gold.skia.org/img/images/d6ac309324273f73bd401d37f860ed63.png">
        </div>
        <a href="https://gold.skia.org/img/images/d6ac309324273f73bd401d37f860ed63.png"
            target=_blank referrer=norel>
            <open-in-new-icon-sk></open-in-new-icon-sk>
        </a>

        <div class=preview>
          <img src="https://gold.skia.org/img/diffs/ad6e15da53efbcd3a1a41b4b86397f76-d6ac309324273f73bd401d37f860ed63.png">
        </div>
        <a href="https://gold.skia.org/img/diffs/ad6e15da53efbcd3a1a41b4b86397f76-d6ac309324273f73bd401d37f860ed63.png"
            target=_blank referrer=norel>
            <open-in-new-icon-sk></open-in-new-icon-sk>
        </a>

        <div class=preview>
          <img src="https://gold.skia.org/img/images/ad6e15da53efbcd3a1a41b4b86397f76.png">
        </div>
        <a href="https://gold.skia.org/img/images/ad6e15da53efbcd3a1a41b4b86397f76.png"
            target=_blank referrer=norel>
            <open-in-new-icon-sk></open-in-new-icon-sk>
        </a>

      </div>
      <button>Zoom</button>
    </div>

    <div>
      <button>Toggle Closest</button>
    </div>

  </div>

  <div>
    <div class=traces>
      This is the traces area. Not mocked out yet.
    </div>
  </div>

  <div class=params>
    <table>
      <thead>
        <tr>
          <th></th>
          <th>2135844182be4...</th>
          <th>Closest Positive</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>alpha_type</td>
          <td>Premul</td>
          <td>Premul</td>
        </tr>
        <tr>
          <td>arch</td>
          <td>arm</td>
          <td>arm</td>
        </tr>
        <tr>
          <td>config</td>
          <td>gles glesdft glesmsaa4</td>
          <td>gles glesdft glesmsaa4</td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
`;


define('digest-details-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

});