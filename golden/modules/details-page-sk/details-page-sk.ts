/**
 * @module module/details-page-sk
 * @description <h2><code>details-page-sk</code></h2>
 *
 * Page to view details about a digest. This includes other digests similar to it and trace history.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../digest-details-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import {
  Commit,
  DetailsRequest,
  DigestDetails,
  GroupingsResponse,
  Params,
  SearchResult,
} from '../rpc_types';

export class DetailsPageSk extends ElementSk {
  private static template = (ele: DetailsPageSk) => {
    if (!ele.didInitialLoad) {
      return html`<h1>Loading...</h1>`;
    }
    if (!ele.grouping?.name) {
      return html`
        <h1>
          Invalid request: Gold now requires the complete "grouping" to be
          specified. This means both the corpus and the test name.
        </h1>
        <div>
          Please fix the query parameters in the URL as following:
          <br />
          /detail?grouping=name%3D<strong>[test_name]</strong>%26source_type%3D<strong>[corpus_name]</strong>&digest=<strong
            >[digest_id]</strong
          >
        </div>
      `;
    }
    if (!ele.details?.digest) {
      const testName = ele.grouping.name;
      return html`
        <div>
          Could not load details for digest ${ele.digest} and test
          "${testName}".
          <br />
          It might not exist or be too new so as not to be indexed yet.
        </div>
      `;
    }
    return html`
      <digest-details-sk
        class="overview"
        .groupings=${ele.groupings}
        .commits=${ele.commits}
        .changeListID=${ele.changeListID}
        .crs=${ele.crs}
        .details=${ele.details}
        @image_compare_size_toggled=${ele.enableFullWidthComparison}>
      </digest-details-sk>
    `;
  };

  private groupings: GroupingsResponse | null = null;

  private grouping: Params = {};

  private digest = '';

  private crs = '';

  private changeListID = '';

  private commits: Commit[] = [];

  private details: SearchResult | null = null;

  private didInitialLoad = false;

  private stateChanged?: () => void;

  // Allows us to abort fetches if we fetch again.
  private fetchController?: AbortController;

  constructor() {
    super(DetailsPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */ () => ({
        // provide empty values
        grouping: this.grouping,
        digest: this.digest,
        changelist_id: this.changeListID,
        crs: this.crs,
      }),
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this.grouping = (newState.grouping as Params) || {};
        this.digest = (newState.digest as string) || '';
        this.changeListID = (newState.changelist_id as string) || '';
        this.crs = (newState.crs as string) || '';
        this.fetchGroupingsOnce();
        this.fetchDigestDetails();
        this._render();
      }
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private async fetchGroupingsOnce() {
    // Only fetch once. We assume this doesn't change during the page's lifetime.
    if (this.groupings) return;

    try {
      sendBeginTask(this);
      this.groupings = await fetch('/json/v1/groupings', {
        method: 'GET',
      }).then(jsonOrThrow);
      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching groupings');
    }
  }

  private async fetchDigestDetails() {
    if (this.fetchController) {
      // Kill any outstanding requests
      this.fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this.fetchController = new AbortController();
    sendBeginTask(this);

    const request: DetailsRequest = {
      digest: this.digest,
      grouping: this.grouping,
    };
    if (this.changeListID && this.crs) {
      request.changelist_id = this.changeListID;
      request.crs = this.crs;
    }

    fetch('/json/v2/details', {
      method: 'POST',
      body: JSON.stringify(request),
      headers: {
        'Content-Type': 'application/json',
      },
      signal: this.fetchController.signal,
    })
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

  private enableFullWidthComparison(e: CustomEvent) {
    e.stopPropagation();
    const digestDetail = this.querySelector('digest-details-sk');

    if (digestDetail && digestDetail.classList) {
      digestDetail.classList.remove('overview');
    }
  }
}

define('details-page-sk', DetailsPageSk);
