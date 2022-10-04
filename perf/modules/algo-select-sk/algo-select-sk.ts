/**
 * @module modules/algo-select-sk
 * @description <h2><code>algo-select-sk</code></h2>
 *
 * Displays and allows changing the clustering algorithm.
 *
 * @evt algo-change - Sent when the algo has changed. The value is stored
 *    in e.detail.algo.
 *
 * @attr {string} algo - The algorithm name.
 */
import 'elements-sk/select-sk';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $ } from 'common-sk/modules/dom';
import { SelectSkSelectionChangedEventDetail } from 'elements-sk/select-sk/select-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ClusterAlgo } from '../json/all';

function toClusterAlgo(s: string): ClusterAlgo {
  const allowed = ['kmeans', 'stepfit'];
  if (allowed.indexOf(s) !== -1) {
    return s as ClusterAlgo;
  }
  return 'kmeans';
}

export interface AlgoSelectAlgoChangeEventDetail {
  algo: ClusterAlgo;
}

export class AlgoSelectSk extends ElementSk {
  constructor() {
    super(AlgoSelectSk.template);
  }

  // TODO(jcgregorio) select-sk needs something like attr-for-selected and
  // fallback-selection like iron-selector.
  private static template = (ele: AlgoSelectSk) => html`
    <select-sk @selection-changed=${ele._selectionChanged}>
      <div
        value="kmeans"
        ?selected=${ele.algo === 'kmeans'}
        title="Use k-means clustering on the trace shapes and look for a step on the cluster centroid."
        >K-Means</div
      >
      <div
        value="stepfit"
        ?selected=${ele.algo === 'stepfit'}
        title="Look for a step in each individual trace."
        >Individual</div
      >
    </select-sk>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('algo');
    this._render();
  }

  attributeChangedCallback(): void {
    this._render();
  }

  static get observedAttributes(): string[] {
    return ['algo'];
  }

  private _selectionChanged(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    let index = e.detail.selection;
    if (index < 0) {
      index = 0;
    }
    this.algo = toClusterAlgo(
      $('div', this)[index].getAttribute('value') || '',
    );
    const detail = {
      algo: this.algo,
    };
    this.dispatchEvent(
      new CustomEvent<AlgoSelectAlgoChangeEventDetail>('algo-change', {
        detail,
        bubbles: true,
      }),
    );
  }

  /** @prop algo {string} The algorithm. */
  get algo(): ClusterAlgo {
    return toClusterAlgo(this.getAttribute('algo') || '');
  }

  set algo(val: ClusterAlgo) {
    this.setAttribute('algo', toClusterAlgo(val));
  }
}

define('algo-select-sk', AlgoSelectSk);
