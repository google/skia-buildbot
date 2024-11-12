/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * Main page of Perf, for exploring data.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import {
  ExploreSimpleSk,
  State as ExploreSimpleSkState,
} from '../explore-simple-sk/explore-simple-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { QueryConfig } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

import '../explore-simple-sk';
import '../favorites-dialog-sk';
import '../test-picker-sk';

import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { errorMessage } from '../errorMessage';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';
import { queryFromKey } from '../paramtools';

export class ExploreSk extends ElementSk {
  private exploreSimpleSk: ExploreSimpleSk | null = null;

  private stateHasChanged: (() => void) | null = null;

  private showMultiViewButton = false;

  private defaults: QueryConfig | null = null;

  private userEmail: string = '';

  private testPicker: TestPickerSk | null = null;

  constructor() {
    super(ExploreSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();
    this._render();

    this.testPicker = this.querySelector('#test-picker');
    this.exploreSimpleSk = this.querySelector('explore-simple-sk');
    this.exploreSimpleSk!.navOpen = true;

    await this.initializeDefaults();

    this.stateHasChanged = stateReflector(
      () => this.exploreSimpleSk!.state as unknown as HintableObject,
      async (hintableState) => {
        const state = hintableState as unknown as ExploreSimpleSkState;
        this.exploreSimpleSk!.openQueryByDefault = true;
        this.exploreSimpleSk!.state = state;
        this.exploreSimpleSk!.useTestPicker = false;
        if (state.use_test_picker_query) {
          await this.initializeTestPicker();
        }
        this.exploreSimpleSk!.render();
      }
    );

    document.addEventListener('keydown', (e) => this.exploreSimpleSk!.keyDown(e));

    this.exploreSimpleSk!.addEventListener('state_changed', () => {
      this.stateHasChanged!();
    });

    this.exploreSimpleSk!.addEventListener('rendered_traces', () => {
      this.showMultiViewButton = true;
      this._render();
    });

    LoggedIn()
      .then((status: LoginStatus) => {
        this.userEmail = status.email;
      })
      .catch(errorMessage);
  }

  private openAddFavoriteDialog = async () => {
    const d = $$<FavoritesDialogSk>('#fav-dialog', this) as FavoritesDialogSk;
    await d!.open();
  };

  private static template = (ele: ExploreSk) => html`
    <div class="explore-padding" ?hidden=${!ele.showMultiViewButton}>
      <favorites-dialog-sk id="fav-dialog"></favorites-dialog-sk>
      <button
        @click=${() => {
          ele.exploreSimpleSk?.viewMultiGraph();
        }}>
        View in multi-graph
      </button>
      <button
        ?disabled=${!ele.userEmail || ele.userEmail === ''}
        @click=${() => {
          ele.openAddFavoriteDialog();
        }}>
        Add to Favorites
      </button>
      <button
        @click=${() => {
          ele.exploreSimpleSk?.toggleGoogleChart();
        }}>
        Toggle Chart Style
      </button>
    </div>
    <test-picker-sk id="test-picker" class="hidden explore-padding"></test-picker-sk>
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
        this.defaults = json;
      });
  }

  // Initialize TestPickerSk
  private async initializeTestPicker() {
    const testPickerParams = this.defaults?.include_params ?? null;
    this.exploreSimpleSk!.useTestPicker = true;
    this.testPicker!.classList.remove('hidden');

    if (this.exploreSimpleSk!.state.queries && this.exploreSimpleSk!.state.queries.length > 0) {
      this.testPicker!.populateFieldDataFromQuery(
        this.exploreSimpleSk!.state.queries.join('&'),
        testPickerParams!
      );
    } else {
      this.testPicker!.initializeTestPicker(
        testPickerParams!,
        this.defaults?.default_param_selections ?? {}
      );
    }

    // Event listener for when the Test Picker plot button is clicked.
    // This will create a new empty Graph at the top and plot it with the
    // selected test values.
    this.addEventListener('plot-button-clicked', (_e) => {
      const explore = this.exploreSimpleSk!;
      if (explore) {
        const query = this.testPicker!.createQueryFromFieldData();
        explore.addFromQueryOrFormula(true, 'query', query, '');
      }
    });

    // Event listener for when the Remove All button is clicked.
    // This will hide the Multiview and favorites buttons since we
    // essentially don't have any graph on the explore page basically
    // rendering these buttons useless.
    this.addEventListener('remove-all', () => {
      this.showMultiViewButton = false;
      this._render();
    });

    // Event listener for when the "Query Highlighted" button is clicked.
    // It will populate the Test Picker with the keys from the highlighted
    // trace.
    this.addEventListener('populate-query', (e) => {
      const trace_key = (e as CustomEvent).detail.key;
      this.testPicker!.populateFieldDataFromQuery(queryFromKey(trace_key), testPickerParams!);
      this.testPicker!.scrollIntoView();
    });
  }
}

define('explore-sk', ExploreSk);
