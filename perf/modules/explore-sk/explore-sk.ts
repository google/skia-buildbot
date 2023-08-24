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

import '../explore-simple-sk';

export class ExploreSk extends ElementSk {
  private exploreSimpleSk: ExploreSimpleSk | null = null;

  private stateHasChanged: (() => void) | null = null;

  constructor() {
    super(ExploreSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.exploreSimpleSk = this.querySelector('explore-simple-sk');
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
  }

  private static template = (ele: ExploreSk) => html`
    <explore-simple-sk></explore-simple-sk>
  `;
}

define('explore-sk', ExploreSk);
