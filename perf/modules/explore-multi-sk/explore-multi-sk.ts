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
import { html } from 'lit-html';
import * as query from '../../../infra-sk/modules/query';
import { define } from '../../../elements-sk/modules/define';
import {
  DEFAULT_RANGE_S,
  ExploreSimpleSk,
  State as ExploreState,
  LabelMode,
} from '../explore-simple-sk/explore-simple-sk';

import { fromKey } from '../paramtools';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../explore-simple-sk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

const GRAPH_LIMIT = 50;

class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  shortcut: string = '';

  showZero: boolean = true;

  dots: boolean = true;

  numCommits: number = 250;

  summary: boolean = false;
}

class GraphConfig {
  formulas: string[] = []; // Formulas

  queries: string[] = []; // Queries

  keys: string = ''; // Keys
}

export class ExploreMultiSk extends ElementSk {
  private graphConfigs: GraphConfig[] = [];

  private exploreElements: ExploreSimpleSk[] = [];

  private stateHasChanged: (() => void) | null = null;

  private _state: State = new State();

  private splitGraphButton: HTMLButtonElement | null = null;

  private mergeGraphsButton: HTMLButtonElement | null = null;

  constructor() {
    super(ExploreMultiSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.splitGraphButton = this.querySelector('#split-graph-button');
    this.mergeGraphsButton = this.querySelector('#merge-graphs-button');

    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      async (hintableState) => {
        const state = hintableState as unknown as State;

        const numElements = this.exploreElements.length;

        let graphConfigs: GraphConfig[] | undefined = [];
        if (state.shortcut !== '') {
          graphConfigs = await this.getConfigsFromShortcut(state.shortcut);
          if (graphConfigs === undefined) {
            graphConfigs = [];
          }
        }

        for (let i = 0; i < graphConfigs.length; i++) {
          if (i >= numElements) {
            this.addEmptyGraph();
          }
          this.graphConfigs[i] = graphConfigs[i];
        }
        while (this.exploreElements.length > graphConfigs.length) {
          this.popGraph();
        }

        this.state = state;
        this.updateButtons();

        this.exploreElements.forEach((elem, i) => {
          const graphConfig = this.graphConfigs[i];

          const newState: ExploreState = {
            formulas: graphConfig.formulas,
            queries: graphConfig.queries,
            keys: graphConfig.keys,
            begin: state.begin,
            end: state.end,
            showZero: state.showZero,
            dots: state.dots,
            numCommits: state.numCommits,
            summary: state.summary,
            xbaroffset: elem.state.xbaroffset,
            autoRefresh: elem.state.autoRefresh,
            requestType: elem.state.requestType,
            pivotRequest: elem.state.pivotRequest,
            sort: elem.state.sort,
            selected: elem.state.selected,
            _incremental: false,
            labelMode: LabelMode.Date,
          };
          elem.state = newState;
        });
      }
    );
  }

  private static template = (ele: ExploreMultiSk) => html`
    <div id="menu">
      <h1>MultiGraph Menu</h1>
      <button
        @click=${() => {
          const explore = ele.addEmptyGraph();
          if (explore) {
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
    </div>
    <hr />
    <div id="graphContainer"></div>
  `;

  private popGraph() {
    const graphDiv: Element | null = this.querySelector('#graphContainer');

    this.exploreElements.pop();
    this.graphConfigs.pop();
    this.updateButtons();
    graphDiv!.removeChild(graphDiv!.lastChild!);
  }

  private clearGraphs() {
    while (this.exploreElements.length > 0) {
      this.popGraph();
    }
  }

  private addEmptyGraph(): ExploreSimpleSk | null {
    if (this.exploreElements.length >= GRAPH_LIMIT) {
      errorMessage(`Cannot exceed display limit of ${GRAPH_LIMIT} graphs.`);
      return null;
    }

    const graphDiv: Element | null = this.querySelector('#graphContainer');
    const explore: ExploreSimpleSk = new ExploreSimpleSk();

    explore.openQueryByDefault = false;
    explore.navOpen = false;
    this.exploreElements.push(explore);
    this.updateButtons();
    this.graphConfigs.push(new GraphConfig());

    const index = this.exploreElements.length - 1;

    explore.addEventListener('state_changed', () => {
      const elemState = explore.state;

      const graphConfig = this.graphConfigs[index];

      graphConfig.formulas = elemState.formulas || [];

      graphConfig.queries = elemState.queries || [];

      graphConfig.keys = elemState.keys || '';

      this.updateShortcut();
    });

    graphDiv!.appendChild(explore);
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
   * Parse a structured key into a queries string.
   *
   * Since this is done on the frontend, this function does not do key or query validation.
   *
   * Example:
   *
   * Key ",a=1,b=2,c=3,"
   *
   * transforms into
   *
   * Query "a=1&b=2&c=3"
   *
   * @param {string} key - A structured trace key.
   *
   * @returns {string} - A query string that can be used in the queries property
   * of explore-simple-sk's state.
   */
  private queryFromKey(key: string): string {
    return new URLSearchParams(fromKey(key)).toString();
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
      const newExplore = this.addEmptyGraph();
      if (newExplore) {
        if (key[0] === ',') {
          const queries = this.queryFromKey(key);
          newExplore.state = {
            ...newExplore.state,
            queries: [queries],
          };
          this.graphConfigs[i].queries = [queries];
        } else {
          const formulas = key;
          newExplore.state = {
            ...newExplore.state,
            formulas: [formulas],
          };
          this.graphConfigs[i].formulas = [formulas];
        }
      }
    });
    this.updateShortcut();
  }

  /**
   *
   * Takes the traces of all Graphs and merges them into a single Graph.
   *
   * Opposite of splitGraph function.
   */
  private async mergeGraphs() {
    const tracesets = this.getTracesets();

    const traces: string[] = [];
    // Flatten tracesets
    tracesets.forEach((traceset) => {
      traceset.forEach((trace) => {
        if (!traces.includes(trace)) {
          traces.push(trace);
        }
      });
    });

    this.clearGraphs();
    const newExplore = this.addEmptyGraph();

    const queries: string[] = [];
    const formulas: string[] = [];

    traces.forEach((trace) => {
      if (trace[0] === ',') {
        queries.push(this.queryFromKey(trace));
      } else {
        formulas.push(trace);
      }
    });
    newExplore!.state = {
      ...newExplore!.state,
      formulas: formulas,
      queries: queries,
    };
    this.graphConfigs[0].formulas = formulas;
    this.graphConfigs[0].queries = queries;
    this.updateShortcut!();
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
  private updateShortcut() {
    if (this.graphConfigs.length === 0) {
      this.state.shortcut = '';
      this.stateHasChanged!();
      return;
    }

    const body = {
      graphs: this.graphConfigs,
    };

    fetch('/_/shortcut/update', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.state.shortcut = json.id;
        this.stateHasChanged!();
      })
      .catch(errorMessage);
  }
}

define('explore-multi-sk', ExploreMultiSk);
