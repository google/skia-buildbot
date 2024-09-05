/**
 * @module module/explore-multi-sk
 * @description <h2><code>explore-multi-sk</code></h2>
 *
 * Page of Perf for exploring data in multiple graphs.
 *
 * User is able to add multiple ExploreSimpleSk instances. The state reflector will only
 * keep track of those properties necessary to add traces to each graph. All other settings,
 * such as the point selected or the beginning and ending range, will be the same
 * for all graphs. For example, passing ?dots=true as a URI parameter will enable
 * dots for all graphs to be loaded. This is to prevent the URL from becoming too long and
 * keeping the logic simple.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import {
  DEFAULT_RANGE_S,
  ExploreSimpleSk,
  State as ExploreState,
  GraphConfig,
  LabelMode,
  updateShortcut,
} from '../explore-simple-sk/explore-simple-sk';

import { TestPickerSk } from '../test-picker-sk/test-picker-sk';

import { queryFromKey } from '../paramtools';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { QueryConfig } from '../json';

import '../explore-simple-sk';
import '../favorites-dialog-sk';
import '../test-picker-sk';
import '../../../golden/modules/pagination-sk/pagination-sk';

import { $$ } from '../../../infra-sk/modules/dom';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';

class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  shortcut: string = '';

  showZero: boolean = false;

  dots: boolean = true;

  numCommits: number = 250;

  summary: boolean = false;

  pageSize: number = 10;

  pageOffset: number = 0;

  totalGraphs: number = 0;

  plotSummary: boolean = false;

  useTestPicker: boolean = false;

  highlight_anomalies: string[] = [];

  enable_chart_tooltip: boolean = false;

  show_remove_all: boolean = true;

  use_titles: boolean = false;
}

export class ExploreMultiSk extends ElementSk {
  private graphConfigs: GraphConfig[] = [];

  private exploreElements: ExploreSimpleSk[] = [];

  private currentPageExploreElements: ExploreSimpleSk[] = [];

  private currentPageGraphConfigs: GraphConfig[] = [];

  private stateHasChanged: (() => void) | null = null;

  private _state: State = new State();

  private splitGraphButton: HTMLButtonElement | null = null;

  private mergeGraphsButton: HTMLButtonElement | null = null;

  private addGraphButton: HTMLButtonElement | null = null;

  private graphDiv: Element | null = null;

  private useTestPicker: boolean = false;

  private testPicker: TestPickerSk | null = null;

  private defaults: QueryConfig | null = null;

  private userEmail: string = '';

  constructor() {
    super(ExploreMultiSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();

    this._render();

    this.graphDiv = this.querySelector('#graphContainer');
    this.splitGraphButton = this.querySelector('#split-graph-button');
    this.mergeGraphsButton = this.querySelector('#merge-graphs-button');
    this.addGraphButton = this.querySelector('#add-graph-button');
    this.testPicker = this.querySelector('#test-picker');

    await this.initializeDefaults();

    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      async (hintableState) => {
        const state = hintableState as unknown as State;

        const numElements = this.exploreElements.length;

        if (state.useTestPicker) {
          this.initializeTestPicker();
        }

        let graphConfigs: GraphConfig[] | undefined = [];
        if (state.shortcut !== '') {
          graphConfigs = await this.getConfigsFromShortcut(state.shortcut);
          if (graphConfigs === undefined) {
            graphConfigs = [];
          }
        }

        // This loop helps get rid of extra graphs that aren't part of the
        // current config. A scenario where this occurs is if we have 1 graph,
        // add another graph and then go back in the browser.
        while (this.exploreElements.length > graphConfigs.length) {
          this.exploreElements.pop();
          this.graphConfigs.pop();
          this.graphDiv!.removeChild(this.graphDiv!.lastChild!);
        }

        for (let i = 0; i < graphConfigs.length; i++) {
          if (i >= numElements) {
            this.addEmptyGraph();
          }
          this.graphConfigs[i] = graphConfigs[i];
        }

        this.state = state;
        this.addGraphsToCurrentPage();

        this.updateButtons();
      }
    );

    LoggedIn()
      .then((status: LoginStatus) => {
        this.userEmail = status.email;
        this._render();
      })
      .catch(errorMessage);
  }

  private canAddFav(): boolean {
    return this.userEmail !== null && this.userEmail !== '';
  }

  private openAddFavoriteDialog = async () => {
    const d = $$<FavoritesDialogSk>('#fav-dialog', this) as FavoritesDialogSk;
    await d!.open();
  };

  private static template = (ele: ExploreMultiSk) => html`
    <div id="menu">
      <h1>MultiGraph Menu</h1>
      <button
        id="add-graph-button"
        @click=${() => {
          const explore = ele.addEmptyGraph();
          if (explore) {
            ele.updatePageForNewExplore();
            explore.openQuery();
          }
        }}
        title="Add empty graph.">
        Add Graph
      </button>
      <button
        id="split-graph-button"
        @click=${() => {
          ele.splitGraph();
        }}
        title="Create multiple graphs from a single graph.">
        Split Graph
      </button>
      <button
        id="merge-graphs-button"
        @click=${() => {
          ele.mergeGraphs();
        }}
        title="Merge all graphs into a single graph.">
        Merge Graphs
      </button>
      <button
        id="favBtn"
        ?disabled=${!ele.canAddFav()}
        @click=${() => {
          ele.openAddFavoriteDialog();
        }}>
        Add to Favorites
      </button>
      <favorites-dialog-sk id="fav-dialog"></favorites-dialog-sk>
      <test-picker-sk id="test-picker" class="hidden"></test-picker-sk>
    </div>
    <hr />

    <pagination-sk
      offset=${ele.state.pageOffset}
      page_size=${ele.state.pageSize}
      total=${ele.state.totalGraphs}
      @page-changed=${ele.pageChanged}>
    </pagination-sk>
    <label>
      <span class="prefix">Charts per page</span>
      <input
        @change=${ele.pageSizeChanged}
        type="number"
        .value="${ele.state.pageSize.toString()}"
        min="1"
        max="50"
        title="The number of charts per page." />
    </label>
    <div id="graphContainer"></div>
    <pagination-sk
      offset=${ele.state.pageOffset}
      page_size=${ele.state.pageSize}
      total=${ele.state.totalGraphs}
      @page-changed=${ele.pageChanged}>
    </pagination-sk>
  `;

  /**
   * Fetch defaults from backend.
   *
   * Defaults are used in multiple ways by downstream elements:
   * - TestPickerSk uses include_params to initialize only the fields
   *   specified and in the given order.
   * - ExploreSimpleSk and TestPickerSk use default_param_selections to
   *   apply default param values to queries before making backend
   *   requests.
   */
  private async initializeDefaults() {
    await fetch(`/_/defaults/`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.defaults = json;
      });

    if (this.defaults !== null) {
      if (
        this.defaults.default_url_values !== undefined &&
        this.defaults.default_url_values !== null
      ) {
        const defaultKeys = Object.keys(this.defaults.default_url_values);
        if (
          defaultKeys !== null &&
          defaultKeys !== undefined &&
          defaultKeys.indexOf('useTestPicker') > -1
        ) {
          const stringToBool = function (str: string): boolean {
            return str.toLowerCase() === 'true';
          };
          this.state.useTestPicker = stringToBool(
            this.defaults!.default_url_values.useTestPicker
          );
        }
      }
    }
  }

  /**
   * Initialize TestPickerSk only if include_params has been specified.
   *
   * If so, hide the default "Add Graph" button and display the Test Picker.
   */
  private async initializeTestPicker() {
    const testPickerParams = this.defaults?.include_params ?? null;
    if (testPickerParams !== null) {
      this.useTestPicker = true;
      this.addGraphButton!.classList.add('hidden');
      this.testPicker!.classList.remove('hidden');
      this.testPicker!.initializeTestPicker(
        testPickerParams!,
        this.defaults?.default_param_selections ?? {}
      );

      // Event listener for when the Test Picker plot button is clicked.
      // This will create a new empty Graph at the top and plot it with the
      // selected test values.
      this.addEventListener('plot-button-clicked', (e) => {
        const explore = this.addEmptyGraph(true);
        if (explore) {
          this.addGraphsToCurrentPage();
          const query = this.testPicker!.createQueryFromFieldData();
          explore.addFromQueryOrFormula(true, 'query', query, '');
        }
      });

      // Event listener for when the "Query Highlighted" button is clicked.
      // It will populate the Test Picker with the keys from the highlighted
      // trace.
      this.addEventListener('populate-query', (e) => {
        const trace_key = (e as CustomEvent).detail.key;
        this.testPicker!.populateFieldDataFromQuery(
          queryFromKey(trace_key),
          testPickerParams!
        );
        this.testPicker!.scrollIntoView();
      });
    }
  }

  private clearGraphs() {
    this.exploreElements = [];
    this.graphConfigs = [];
    this.updateButtons();
  }

  private emptyCurrentPage(): void {
    while (this.graphDiv!.hasChildNodes()) {
      this.graphDiv!.removeChild(this.graphDiv!.lastChild!);
    }
    this.currentPageExploreElements = [];
    this.currentPageGraphConfigs = [];
  }

  private addGraphsToCurrentPage(): void {
    this.state.totalGraphs = this.exploreElements.length;
    this.emptyCurrentPage();
    const startIndex = this.state.pageOffset;
    let endIndex = startIndex + this.state.pageSize - 1;
    if (this.exploreElements.length <= endIndex) {
      endIndex = this.exploreElements.length - 1;
    }

    for (let i = startIndex; i <= endIndex; i++) {
      this.graphDiv!.appendChild(this.exploreElements[i]);
      this.currentPageExploreElements.push(this.exploreElements[i]);
      this.currentPageGraphConfigs.push(this.graphConfigs[i]);
    }

    this.currentPageExploreElements.forEach((elem, i) => {
      const graphConfig = this.currentPageGraphConfigs[i];
      this.addStateToExplore(elem, graphConfig);
    });

    this._render();
  }

  private addStateToExplore(
    explore: ExploreSimpleSk,
    graphConfig: GraphConfig
  ) {
    const newState: ExploreState = {
      formulas: graphConfig.formulas || [],
      queries: graphConfig.queries || [],
      keys: graphConfig.keys || '',
      begin: this.state.begin,
      end: this.state.end,
      showZero: this.state.showZero,
      dots: this.state.dots,
      numCommits: this.state.numCommits,
      summary: this.state.summary,
      xbaroffset: explore.state.xbaroffset,
      autoRefresh: explore.state.autoRefresh,
      requestType: explore.state.requestType,
      pivotRequest: explore.state.pivotRequest,
      sort: explore.state.sort,
      selected: explore.state.selected,
      _incremental: false,
      labelMode: LabelMode.Date,
      disable_filter_parent_traces: explore.state.disable_filter_parent_traces,
      plotSummary: this.state.plotSummary,
      highlight_anomalies: this.state.highlight_anomalies,
      enable_chart_tooltip: this.state.enable_chart_tooltip,
      show_remove_all: this.state.show_remove_all,
      use_titles: this.state.use_titles,
      useTestPicker: this.state.useTestPicker,
      use_test_picker_query: false,
    };
    explore.state = newState;
  }

  private addEmptyGraph(unshift?: boolean): ExploreSimpleSk | null {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(
      true,
      this.useTestPicker
    );
    const graphConfig = new GraphConfig();
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;
    if (unshift) {
      this.exploreElements.unshift(explore);
      this.graphConfigs.unshift(graphConfig);
    } else {
      this.exploreElements.push(explore);
      this.graphConfigs.push(graphConfig);
    }
    this.updateButtons();

    explore.addEventListener('state_changed', () => {
      const elemState = explore.state;

      graphConfig.formulas = elemState.formulas || [];

      graphConfig.queries = elemState.queries || [];

      graphConfig.keys = elemState.keys || '';

      this.updateShortcutMultiview();
    });

    return explore;
  }

  public get state(): State {
    return this._state;
  }

  public set state(v: State) {
    this._state = v;
  }

  private updateButtons() {
    if (this.exploreElements.length === 1) {
      this.splitGraphButton!.disabled = false;
    } else {
      this.splitGraphButton!.disabled = true;
    }

    if (this.exploreElements.length > 1) {
      this.mergeGraphsButton!.disabled = false;
    } else {
      this.mergeGraphsButton!.disabled = true;
    }
  }

  /**
   * Get the trace keys for each graph formatted in a 2D string array.
   *
   * In the case of formulas, we extract the base key, since all the operations
   * we will do on these keys (adding to shortcut store, merging into single graph, etc.)
   * are not applicable to function strings. Currently does not support query-based formulas
   * (e.g. count(filter("a=b"))).
   *
   * TODO(@eduardoyap): add support for query-based formulas.
   *
   * Example output given that we're displaying 2 graphs:
   *
   * [
   *  [
   *    ",a=1,b=2,c=3,",
   *    ",a=1,b=2,c=4,"
   *  ],
   *  [
   *    ",a=1,b=2,c=5,"
   *  ]
   * ]
   *
   * @returns {string[][]} - Trace keys.
   */
  private getTracesets(): string[][] {
    const tracesets: string[][] = [];

    this.exploreElements.forEach((elem) => {
      const traceset: string[] = [];

      const formula_regex = new RegExp(/\((,[^)]+,)\)/);
      // Tracesets include traces from Queries and Keys. Traces
      // from formulas are wrapped around a formula string.
      Object.keys(elem.getTraceset()).forEach((key) => {
        if (key[0] === ',') {
          traceset.push(key);
        } else {
          const match = formula_regex.exec(key);
          if (match) {
            traceset.push(match[1]);
          }
        }
      });
      if (traceset.length !== 0) {
        tracesets.push(traceset);
      }
    });
    return tracesets;
  }

  /**
   * Takes the traces of a single graph and create a separate graph for each of those
   * traces.
   *
   * Say the displayed graphs are of the following form:
   *
   * [
   *  [
   *    ",a=1,b=2,c=3,",
   *    ",a=1,b=2,c=4,",
   *    ",a=1,b=2,c=5,"
   *  ],
   * ]
   *
   * The resulting Multigraph structure will be:
   *
   * [
   *  [
   *    ",a=1,b=2,c=3,"
   *  ],
   *  [
   *    ",a=1,b=2,c=4,"
   *  ],
   *  [
   *    ",a=1,b=2,c=5,"
   *  ]
   * ]
   *
   *
   */
  private async splitGraph() {
    const traceset = this.getTracesets()[0];
    if (!traceset) {
      return;
    }
    this.clearGraphs();
    traceset.forEach((key, i) => {
      this.addEmptyGraph();
      if (key[0] === ',') {
        const queries = queryFromKey(key);
        this.graphConfigs[i].queries = [queries];
      } else {
        const formulas = key;
        this.graphConfigs[i].formulas = [formulas];
      }
    });
    this.updateShortcutMultiview();

    // Upon the split action, we would want to move to the first page
    // of the split graph set.
    this.state.pageOffset = 0;
    this.addGraphsToCurrentPage();
  }

  /**
   *
   * Takes the traces of all Graphs and merges them into a single Graph.
   *
   * Opposite of splitGraph function.
   */
  private async mergeGraphs() {
    const mergedGraphConfig = new GraphConfig();
    this.graphConfigs.forEach((config) => {
      config.formulas.forEach((formula) => {
        mergedGraphConfig.formulas.push(formula);
      });
      config.queries.forEach((query) => {
        mergedGraphConfig.queries.push(query);
      });
    });
    this.clearGraphs();
    this.addEmptyGraph();

    this.graphConfigs[0] = mergedGraphConfig;
    this.updateShortcutMultiview!();
    // Upon the merge action, we would want to move to the first page.
    this.state.pageOffset = 0;
    this.addGraphsToCurrentPage();
  }

  /**
   * Fetches the Graph Configs in the DB for a given shortcut ID.
   *
   * @param {string} shortcut - shortcut ID to look for in the GraphsShortcut table.
   * @returns - List of Graph Configs matching the shortcut ID in the GraphsShortcut table
   * or undefined if the ID doesn't exist.
   */
  private getConfigsFromShortcut(
    shortcut: string
  ): Promise<GraphConfig[]> | undefined {
    const body = {
      ID: shortcut,
    };

    return fetch('/_/shortcut/get', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => json.graphs)
      .catch(errorMessage);
  }

  /**
   * Creates a shortcut ID for the current Graph Configs and updates the state.
   *
   */
  private updateShortcutMultiview() {
    updateShortcut(this.graphConfigs)
      .then((shortcut) => {
        if (shortcut === '') {
          this.state.shortcut = '';
          this.stateHasChanged!();
          return;
        }
        this.state.shortcut = shortcut;
        this.stateHasChanged!();
      })
      .catch(errorMessage);
  }

  private pageChanged(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    this.state.pageOffset = Math.max(
      0,
      this.state.pageOffset + e.detail.delta * this.state.pageSize
    );
    this.stateHasChanged!();
    this.addGraphsToCurrentPage();
  }

  private pageSizeChanged(e: MouseEvent) {
    this.state.pageSize = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged!();
    this.addGraphsToCurrentPage();
  }

  private updatePageForNewExplore() {
    // Check if there is space left on the current page
    if (this.graphDiv!.childElementCount === this.state.pageSize) {
      // We will have to add another page since the current one is full.
      // Go to the next page.
      this.pageChanged(
        new CustomEvent<PaginationSkPageChangedEventDetail>('page-changed', {
          detail: {
            delta: 1,
          },
          bubbles: true,
        })
      );
    } else {
      // Re-render the page
      this.addGraphsToCurrentPage();
    }
  }
}

define('explore-multi-sk', ExploreMultiSk);
