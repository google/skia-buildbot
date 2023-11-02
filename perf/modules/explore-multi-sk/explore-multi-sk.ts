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

import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../explore-simple-sk';

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

  constructor() {
    super(ExploreMultiSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

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
    <div id="graphContainer"></div>
    <button
      @click=${() => {
        const explore = ele.addEmptyGraph();
        explore.openQuery();
      }}
      title="Add empty graph.">
      Add Graph
    </button>
  `;

  private popGraph() {
    const graphDiv: Element | null = this.querySelector('#graphContainer');

    this.exploreElements.pop();
    this.graphConfigs.pop();
    graphDiv!.removeChild(graphDiv!.lastChild!);
  }

  private addEmptyGraph(): ExploreSimpleSk {
    const graphDiv: Element | null = this.querySelector('#graphContainer');
    const explore: ExploreSimpleSk = new ExploreSimpleSk();

    explore.openQueryByDefault = false;
    this._state.numGraphs += 1;
    this.exploreElements.push(explore);
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
}

define('explore-multi-sk', ExploreMultiSk);
