/**
 * @module module/details-page-sk
 * @description <h2><code>details-page-sk</code></h2>
 *
 * Page to view details about a digest. This includes other digests similar to it and trace history.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../digest-details-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { Commit, DigestDetails, SearchResult } from '../rpc_types';

export class DetailsPageSk extends ElementSk {
  private static template = (ele: DetailsPageSk) => {
    if (!ele.didInitialLoad) {
      return html`<h1>Loading...</h1>`;
    }
    if (!ele.details?.digest) {
      return html`
        <div>
          Could not load details for digest ${ele.digest} and test "${ele.grouping}".
          <br>
          It might not exist or be too new so as not to be indexed yet.
        </div>
      `;
    }
    return html`
      <digest-details-sk .commits=${ele.commits}
                         .changeListID=${ele.changeListID}
                         .crs=${ele.crs}
                         .details=${ele.details}
                         .useNewAPI=${ele.useNewAPI}>
      </digest-details-sk>
    `;
  };

  private grouping = '';

  private digest = '';

  private crs = '';

  private changeListID = '';

  private commits: Commit[] = [];

  private details: SearchResult | null = null;

  private didInitialLoad = false;

  private useNewAPI: boolean = false;

  private stateChanged?: ()=> void;

  // Allows us to abort fetches if we fetch again.
  private fetchController?: AbortController;

  constructor() {
    super(DetailsPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        test: this.grouping, // TODO(kjlubick) rename test -> grouping
        digest: this.digest,
        changelist_id: this.changeListID,
        crs: this.crs,
        use_new_api: this.useNewAPI,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this.grouping = newState.test as string || '';
        this.digest = newState.digest as string || '';
        this.changeListID = newState.changelist_id as string || '';
        this.crs = newState.crs as string || '';
        this.useNewAPI = (newState.use_new_api as boolean) || false;
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
    if (this.fetchController) {
      // Kill any outstanding requests
      this.fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this.fetchController = new AbortController();
    const extra = {
      signal: this.fetchController.signal,
    };
    sendBeginTask(this);
    const urlBase = this.useNewAPI ? '/json/v2/details' : '/json/v1/details';

    const url = `${urlBase}?test=${encodeURIComponent(this.grouping)}`
      + `&digest=${encodeURIComponent(this.digest)}&changelist_id=${this.changeListID}`
      + `&crs=${this.crs}`;
    fetch(url, extra)
      .then(jsonOrThrow)
      .then((digestDetails: DigestDetails) => {
        this.commits = digestDetails.commits || [];
        this.details = digestDetails.digest;
        this.didInitialLoad = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => {
        this.commits = [];
        this.details = null;
        this.didInitialLoad = true;
        this._render();
        sendFetchError(this, e, 'digest-details');
      });
  }
}

define('details-page-sk', DetailsPageSk);
