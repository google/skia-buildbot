/**
 * @module modules/byblame-page-sk/byblameentry-sk
 * @description <h2><code>byblameentry-sk</code></h2>
 *
 * Displays a
 * [ByBlameEntry]{@link https://github.com/google/skia-buildbot/blob/0e14ae66aa226821e981bbd4c63dc8d07776997a/golden/go/web/web.go#L318},
 * that is, a blame group of untriaged digests.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { baseRepoURL } from '../settings';

const template = (el) => html`
<div class=blame>
  <p>
    <a href=${el._blameHref()} class=triage target=_blank rel=noopener>
      ${el.byBlameEntry.nDigests === 1
    ? '1 untriaged digest'
    : `${el.byBlameEntry.nDigests} untriaged digests`}
    </a>
  </p>

  ${!el.byBlameEntry.commits || el.byBlameEntry.commits.length === 0
    ? html`<p class=no-blamelist>No blamelist.</p>`
    : blameListTemplate(el)}

  <h3>Tests affected</h3>
  <p class=num-tests-affected>
    ${el.byBlameEntry.nTests === 1
    ? '1 test affected.'
    : `${el.byBlameEntry.nTests} tests affected.`}
  </p>

  ${affectedTestsTemplate(el.byBlameEntry.affectedTests)}
</div>
`;

const blameListTemplate = (el) => html`
<h3>Blame</h3>

<ul class=blames>
  ${el.byBlameEntry.commits.map(
    (commit) => html`
          <li>
            <a href=${el._commitHref(commit.hash)}
               target=_blank
               rel=noopener>
              ${commit.hash.slice(0, 7)}
            </a>
            <span class=commit-message>
              ${commit.message}
            </span>
            <br/>
            <small>
              <span class=author>${commit.author}</span>,
              <span class=age>
                ${diffDate(commit.commit_time * 1000)}
              </span> ago.
            </small>
          </li>`,
  )}
</ul>`;

const affectedTestsTemplate = (affectedTests) => (!affectedTests || affectedTests.length === 0
  ? ''
  : html`
<table class=affected-tests>
  <thead>
    <tr>
      <th>Test</th>
      <th># Digests</th>
      <th>Example</th>
    </tr>
  </thead>
  <tbody>
    ${affectedTests.map(
    (test) => html`
            <tr>
              <td class=test>${test.test}</td>
              <td class=num-digests>${test.num}</td>
              <td>
                <a href=${detailHref(test)}
                   class=example-link
                   target=_blank
                   rel=noopener>
                  ${test.sample_digest}
                </a>
              </td>
            </tr>`,
  )}
  </tbody>
</table>`);

const detailHref = (test) => `/detail?test=${test.test}&digest=${test.sample_digest}`;

define('byblameentry-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop byBlameEntry {Object} A ByBlameEntry object returned by the
   *     /json/byblame RPC endpoint.
   */
  get byBlameEntry() { return this._byBlameEntry; }

  set byBlameEntry(v) {
    this._byBlameEntry = v;
    this._render();
  }

  /** @prop corpus {string} The corpus corresponding to this blame group. */
  get corpus() { return this._corpus; }

  set corpus(v) {
    this._corpus = v;
    this._render();
  }

  _blameHref() {
    const query = encodeURIComponent(`source_type=${this.corpus}`);
    const groupID = this.byBlameEntry.groupID;
    return `/search?blame=${groupID}&unt=true&head=true&query=${query}`;
  }

  _commitHref(hash) {
    const repo = baseRepoURL();
    if (!hash || !repo) {
      return '';
    }
    const path = repo.indexOf('github.com') !== -1 ? 'commit' : '+show';
    return `${repo}/${path}/${hash}`;
  }
});
