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
import { ByBlameEntry, Commit, TestRollup } from '../rpc_types';

const MAX_COMMITS = 10;

const commitHref = (hash: string) => {
  const repo = baseRepoURL();
  if (!hash || !repo) {
    return '';
  }
  const path = repo.indexOf('github.com') !== -1 ? 'commit' : '+show';
  return `${repo}/${path}/${hash}`;
};

const detailHref = (test: TestRollup, useNewAPI: boolean) => {
  if (useNewAPI) {
    return `/detail?test=${test.test}&digest=${test.sample_digest}&use_new_api=true`;
  }
  return `/detail?test=${test.test}&digest=${test.sample_digest}`;
};

export class ByBlameEntrySk extends ElementSk {
  private static template = (el: ByBlameEntrySk) => html`
    <div class=blame>
      <p>
        <a href=${el.blameHref()} class=triage target=_blank rel=noopener>
          ${el.byBlameEntry!.nDigests === 1
    ? '1 untriaged digest'
    : `${el.byBlameEntry!.nDigests} untriaged digests`}
        </a>
      </p>

      ${ByBlameEntrySk.blameListTemplate(el.byBlameEntry?.commits)}

      <h3>Tests affected</h3>
      <p class=num-tests-affected>
        ${el.byBlameEntry!.nTests === 1
      ? '1 test affected.'
      : `${el.byBlameEntry!.nTests} tests affected.`}
      </p>

      ${ByBlameEntrySk.affectedTestsTemplate(el.byBlameEntry?.affectedTests, el.useNewAPI)}
    </div>
  `;

  private static blameListTemplate = (commits?: Commit[] | null) => {
    if (!commits || commits.length === 0) {
      return html`<p class=no-blamelist>No blamelist.</p>`;
    }
    const andNMore = commits.length - MAX_COMMITS;

    commits = commits.slice(0, MAX_COMMITS);

    return html`
      <h3>Blame</h3>

      <ul class=blames>
        ${commits.map((commit) => html`
          <li>
            <a href=${commitHref(commit.hash)} target=_blank rel=noopener>
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
          </li>`)}
          ${andNMore > 0 ? html`<li>And ${andNMore} other commit(s)</li>` : ''}
      </ul>`;
  };

  private static affectedTestsTemplate = (affectedTests: TestRollup[] | undefined | null, useNewAPI: boolean) => {
    if (!affectedTests || affectedTests.length === 0) return '';
    return html`
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
                      <a href=${detailHref(test, useNewAPI)}
                         class=example-link
                         target=_blank
                         rel=noopener>
                        ${test.sample_digest}
                      </a>
                    </td>
                  </tr>`,
    )}
        </tbody>
      </table>
    `;
  }

  private _byBlameEntry: ByBlameEntry | null = null;

  private _corpus = '';

  private _useNewAPI = false;

  constructor() {
    super(ByBlameEntrySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** A ByBlameEntry object returned by the /json/v1/byblame RPC endpoint. */
  get byBlameEntry(): ByBlameEntry | null { return this._byBlameEntry; }

  set byBlameEntry(v: ByBlameEntry | null) {
    this._byBlameEntry = v;
    this._render();
  }

  /** The corpus corresponding to this blame group. */
  get corpus(): string { return this._corpus; }

  set corpus(v: string) {
    this._corpus = v;
    this._render();
  }

  get useNewAPI(): boolean { return this._useNewAPI; }

  set useNewAPI(b: boolean) {
    this._useNewAPI = b;
    this._render();
  }

  private blameHref() {
    const blameID = this.byBlameEntry!.groupID;

    const newAPI = this.useNewAPI ? '&use_new_api=true' : '';

    return `/search?blame=${blameID}&corpus=${this.corpus}${newAPI}`;
  }
}

define('byblameentry-sk', ByBlameEntrySk);
