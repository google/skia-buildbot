/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * Main page of Perf, for exploring data.
 */
import { html } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ExploreSimpleSk, State } from '../explore-simple-sk/explore-simple-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { QueryConfig } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

import '../explore-simple-sk';

export class ExploreSk extends ElementSk {
  private exploreSimpleSk: ExploreSimpleSk | null = null;

  private stateHasChanged: (() => void) | null = null;

  private showMultiViewButton = false;

  private defaults: QueryConfig | null = null;

  constructor() {
    super(ExploreSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();
    this._render();

    this.exploreSimpleSk = this.querySelector('explore-simple-sk');
    this.exploreSimpleSk!.openQueryByDefault = true;
    this.exploreSimpleSk!.navOpen = true;

    await this.initializeDefaults();

    this.stateHasChanged = stateReflector(
      () => this.exploreSimpleSk!.state as unknown as HintableObject,
      (hintableState) => {
        const state = hintableState as unknown as State;
        this.exploreSimpleSk!.state = state;
      }
    );

    document.addEventListener('keydown', (e) =>
      this.exploreSimpleSk!.keyDown(e)
    );

    this.exploreSimpleSk!.addEventListener('state_changed', () => {
      this.stateHasChanged!();
    });

    this.exploreSimpleSk!.addEventListener('rendered_traces', () => {
      this.showMultiViewButton = true;
      this._render();
    });
  }

  private static template = (ele: ExploreSk) => html`
    <div ?hidden=${!ele.showMultiViewButton}>
      <button
        style="margin: 16px 0 0 16px;"
        @click=${() => {
          ele.exploreSimpleSk?.viewMultiGraph();
        }}>
        View in multi-graph
      </button>
    </div>
    <explore-simple-sk></explore-simple-sk>
  `;

  /**
   * Fetches defaults from backend and passes them down to the
   * ExploreSimpleSk element.
   */
  private async initializeDefaults() {
    await fetch(`/_/defaults/`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.exploreSimpleSk!.defaults = json;
      });
  }
}

define('explore-sk', ExploreSk);
