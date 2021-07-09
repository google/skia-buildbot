/**
 * @module modules/byblame-page-sk
 * @description <h2><code>byblame-page-sk</code></h2>
 *
 * Displays the current untriaged digests, grouped by the commits that may have caused them
 * (i.e. the blamelist or blame, for short).
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../byblameentry-sk';
import '../corpus-selector-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';
import {
  ByBlameEntry, ByBlameResponse, GUICorpusStatus, StatusResponse,
} from '../rpc_types';

const corpusRendererFn = (corpus: GUICorpusStatus): string => {
  if (corpus.untriagedCount) {
    return `${corpus.name} (${corpus.untriagedCount})`;
  }
  return corpus.name;
};

export class ByBlamePageSk extends ElementSk {
  private static template = (ele: ByBlamePageSk) => html`
    <div class=top-container>
      <corpus-selector-sk
          .corpora=${ele.corpora}
          .selectedCorpus=${ele.corpora.find((c) => c.name === ele.corpus)}
          .corpusRendererFn=${corpusRendererFn}
          @corpus-selected=${ele.handleCorpusChange}>
      </corpus-selector-sk>
    </div>

    <div class=entries>
      ${(!ele.entries || ele.entries.length === 0)
    ? ByBlamePageSk.noEntries(ele)
    : ele.entries.map((entry) => ByBlamePageSk.entryTemplate(ele, entry))}
    </div>
  `;

  private static entryTemplate = (ele: ByBlamePageSk, entry: ByBlameEntry) => html`
    <byblameentry-sk
        .byBlameEntry=${entry}
        .corpus=${ele.corpus}>
    </byblameentry-sk>
  `;

  private static noEntries = (ele: ByBlamePageSk) => {
    if (!ele.loaded) {
      return 'Loading untriaged digests...';
    }
    return `No untriaged digests for corpus ${ele.corpus}.`;
  };

  private corpora: GUICorpusStatus[] = [];

  private corpus = '';

  private entries: ByBlameEntry[] = [];

  private loaded = false;

  private useNewAPI = false;

  private readonly stateChanged: ()=> void;

  private fetchController: AbortController | null = null;

  constructor() {
    super(ByBlamePageSk.template);

    // stateReflector will trigger on DomReady.
    this.stateChanged = stateReflector(
      /* getState */ () => ({
        // Provide empty values.
        corpus: this.corpus,
        use_new_api: this.useNewAPI || '',
      }),
      /* setState */ (newState) => {
        // The stateReflector's lingering popstate event handler will continue
        // to call this function on e.g. browser back button clicks long after
        // this custom element is detached from the DOM.
        if (!this._connected) {
          return;
        }

        this.corpus = newState.corpus as string || defaultCorpus();
        this.useNewAPI = (newState.use_new_api as boolean) || false;
        this._render(); // Update corpus selector immediately.
        this.fetch();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    // Show loading indicator while we wait for results from the server.
    this._render();
  }

  private handleCorpusChange(event: CustomEvent<GUICorpusStatus>) {
    this.corpus = event.detail.name;
    this.stateChanged();
    this.fetch();
  }

  private fetch() {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this.fetchController) {
      this.fetchController.abort();
    }
    this.fetchController = new AbortController();

    const options = {
      method: 'GET',
      signal: this.fetchController.signal,
    };

    const query = encodeURIComponent(`source_type=${this.corpus}`);
    let byBlameURL = `/json/v1/byblame?query=${query}`;
    if (this.useNewAPI) {
      byBlameURL = `/json/v2/byblame?query=${query}`;
    }

    sendBeginTask(this);
    fetch(byBlameURL, options)
      .then(jsonOrThrow)
      .then((res: ByBlameResponse) => {
        this.entries = res.data || [];
        this.loaded = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'byblamepage_entries'));

    sendBeginTask(this);
    const url = this.useNewAPI ? '/json/v2/trstatus' : '/json/v1/trstatus';
    fetch(url, options)
      .then(jsonOrThrow)
      .then((res: StatusResponse) => {
        this.corpora = res.corpStatus;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'byblamepage_trstatus'));
  }
}

define('byblame-page-sk', ByBlamePageSk);
