/**
 * @module module/diff-page-sk
 * @description <h2><code>diff-page-sk</code></h2>
 *
 * Page to view a specific diff between two digests. This does not include trace data.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../digest-details-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import {DigestComparison, LeftDiffInfo, SRDiffDigest} from '../rpc_types';

export class DiffPageSk extends ElementSk {
  private static template = (ele: DiffPageSk) => {
    if (!ele.didInitialLoad) {
      return html`<p>Loading...</p>`;
    }
    if (!ele.leftDetails) {
      return html`<p>Could not load diff.</p>`;
    }
    return html`
      <digest-details-sk .details=${ele.leftDetails}
                         .right=${ele.rightDetails}
                         .changeListID=${ele.changeListID}
                         .crs=${ele.crs}>
      </digest-details-sk>
    `;
  };

  private grouping = '';
  private leftDigest = '';
  private rightDigest = '';
  private crs = '';
  private changeListID = '';
  private leftDetails: LeftDiffInfo | null = null;
  private rightDetails: SRDiffDigest | null = null;
  private useSQL = false;
  private didInitialLoad = false;

  private readonly _stateChanged: () => void;

  // Allows us to abort fetches if we fetch again.
  private _fetchController?: AbortController;

  constructor() {
    super(DiffPageSk.template);

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        test: this.grouping, // TODO(kjlubick) rename test -> grouping
        left: this.leftDigest,
        right: this.rightDigest,
        use_sql: this.useSQL,
        changelist_id: this.changeListID,
        crs: this.crs,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this.grouping = newState.test as string || '';
        this.leftDigest = newState.left as string || '';
        this.rightDigest = newState.right as string || '';
        this.changeListID = newState.changelist_id as string || '';
        this.crs = newState.crs as string || '';
        this.useSQL = newState.use_sql as boolean || false;
        this.fetch();
        this._render();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private fetch() {
    // Kill any outstanding requests
    this._fetchController?.abort();

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    const extra = {
      signal: this._fetchController.signal,
    };
    sendBeginTask(this);

    let url = `/json/v1/diff?test=${encodeURIComponent(this.grouping)}`
      + `&left=${encodeURIComponent(this.leftDigest)}`
      + `&right=${encodeURIComponent(this.rightDigest)}`
      + `&changelist_id=${this.changeListID}&crs=${this.crs}`;
    if (this.useSQL) {
      url += '&use_sql=true';
    }

    fetch(url, extra)
      .then(jsonOrThrow)
      .then((obj: DigestComparison) => {
        this.leftDetails = obj.left;
        this.rightDetails = obj.right;
        this.didInitialLoad = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => {
        this.leftDetails = null;
        this.rightDetails = null;
        this.didInitialLoad = false;
        this._render();
        sendFetchError(this, e, 'diff-details');
      });
  }
}

define('diff-page-sk', DiffPageSk);
