import * as human from 'common-sk/modules/human'

import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import { html } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'


const _changelist = (cl) => html`
<tr>
  <td>
    <a title="See codereview in a new window" target="_blank" rel="noopener" href="${cl.url}">
      ${cl.id}
    </a>
  </td>
  <td>${cl.owner}</td>
  <td title=${cl.updated}>${human.diffDate(cl.updated)} ago</td>
  <td>${cl.subject}</td>
</tr>`;

const template = (ele) => html`
<div>pagination here</div>

<table>
  <thead>
    <tr>
      <th>Issue</th>
      <th>Owner</th>
      <th>Updated</th>
      <th>Subject</th>
    </tr>
  </thead>
  <tbody>
${ele._cls.map(_changelist)}
</tbody>
`;

define('changelists-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._cls = [];

    // Allows us to abort fetches if a user pages.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback()
    this._render();
    // Fetch the data on the next microtasks - this makes
    // sure our mocks are set up when running locally.
    setTimeout(() => this._fetch());
  }

  _fetch() {
    if (this._fetchController) {
      // Kill any outstanding requests
      this._fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    const extra = {
      signal: this._fetchController.signal,
    };

    fetch(`/json/changelists`, extra)
      .then(jsonOrThrow)
      .then((json) => {
        console.log(json)
        this._cls = json.data;
        this._render()
      })
      .catch((e) => this._fetchError(e, 'changelists'));
  }

  /** Handles a fetch error
      @param {Object} e The error given by fetch.
      @param {String} loadingWhat A short string to describe what failed.
                      (e.g. bots/list if the bots/list endpoint was queried)
   */
  _fetchError(e, loadingWhat) {
    if (e.name !== 'AbortError') {
      // We can ignore AbortError since they fire anytime we page.
      // Chrome and Firefox report a DOMException in this case:
      // https://developer.mozilla.org/en-US/docs/Web/API/DOMException
      console.error(e);
      errorMessage(`Unexpected error loading ${loadingWhat}: ${e.message}`,
                   5000);
    }
  }

});