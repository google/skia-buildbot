/**
 * @module modules/chromium-build-selector-sk
 * @description A custom element for selecting from available chromium builds.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { fromObject } from 'common-sk/modules/query';
import { errorMessage } from 'elements-sk/errorMessage';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';

import { chromiumBuildDescription } from '../ctfe_utils';

const template = (ele) => html`
<div>
<select-sk>
  ${ele._builds.map((b) => (html`<div>${chromiumBuildDescription(b)}</div>`))}
</select-sk>
</div>
`;

define('chromium-build-selector-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._builds = [];
  }

  connectedCallback() {
    super.connectedCallback();
    const queryParams = {
      size: 20,
      successful: true,
    };
    const url = `/_/get_chromium_build_tasks?${fromObject(queryParams)}`;
    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        this._builds = json.data;
        this._render();
        $$('select-sk', this).selection = 0;
      })
      .catch(errorMessage);
    this._render();
  }

  /**
   * @prop {string} build - The build selected.
   */
  get build() {
    return this._builds[$$('select-sk', this).selection];
  }
});
