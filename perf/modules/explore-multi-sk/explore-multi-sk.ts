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
} from '../explore-simple-sk/explore-simple-sk';

import { fromKey } from '../paramtools';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../explore-simple-sk';

const GRAPH_LIMIT = 50;

class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  numGraphs: number = 0; // Let's state reflector know how many graphs to add.

  graphConfigs: string[] = [];

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
      (hintableState) => {
        const state = hintableState as unknown as State;

        const numElements = this.exploreElements.length;

        for (let i = 0; i < state.numGraphs; i++) {
          if (i >= numElements) {
            this.addEmptyGraph();
          }
        }
        while (this.exploreElements.length > state.numGraphs) {
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
            // TODO(seanmccullough): Make sure this feature works for /m/ too.
            _incremental: false,
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
    this._state.numGraphs -= 1;
    this.updateButtons();
    graphDiv!.removeChild(graphDiv!.lastChild!);
  }

  private clearGraphs() {
    while (this.exploreElements.length > 0) {
      this.popGraph();
    }
    this._state.numGraphs = 0;
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
    this._state.numGraphs += 1;
    this.updateButtons();
    this.graphConfigs.push(new GraphConfig());

    const index = this.exploreElements.length - 1;

    explore.addEventListener('state_changed', () => {
      const elemState = explore.state;

      const graphConfig = this.graphConfigs[index];

      graphConfig.formulas = elemState.formulas || [];

      graphConfig.queries = elemState.queries || [];

      graphConfig.keys = elemState.keys || '';

      this.stateHasChanged!();
    });

    graphDiv!.appendChild(explore);
    return explore;
  }

  public get state(): State {
    const graphConfigs: string[] = [];

    this.graphConfigs.forEach((config) => {
      graphConfigs.push(query.fromObject(config as unknown as HintableObject));
    });

    return {
      ...this._state,
      graphConfigs: graphConfigs,
    };
  }

  public set state(v: State) {
    v.graphConfigs.forEach((config, i) => {
      const hintConfig = this.graphConfigs[i] as unknown as HintableObject;
      const parsedConfig = query.toObject(
        config,
        hintConfig
      ) as unknown as GraphConfig;
      this.graphConfigs[i] = {
        ...this.graphConfigs[i],
        ...parsedConfig,
      };
    });

    this._state = v;
  }

  private updateButtons() {
    if (this._state.numGraphs === 1) {
      this.splitGraphButton!.disabled = false;
    } else {
      this.splitGraphButton!.disabled = true;
    }

    if (this._state.numGraphs > 1) {
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
    this.stateHasChanged!();
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
    this.stateHasChanged!();
  }
}

define('explore-multi-sk', ExploreMultiSk);
