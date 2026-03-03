/**
 * @module modules/graph-list-sk
 * @description <h2><code>graph-list-sk</code></h2>
 */
import { html, LitElement, PropertyValues } from 'lit';
import { property, state, query } from 'lit/decorators.js';
import { define } from '../../../elements-sk/modules/define';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import '../../../elements-sk/modules/spinner-sk';

export interface GraphItem {
  id: string;
  generateGraph: () => ExploreSimpleSk | null;
}

export class GraphListSk extends LitElement {
  @property({ attribute: false }) items: GraphItem[] = [];

  @property({ type: Number }) chunkSize: number = 5;

  @property({ type: Number }) visibleLimit: number = 25;

  @state() private _renderedCount: number = 0;

  @state() private _currentlyLoading: string = '';

  @state() private _allGraphsLoaded: boolean = true;

  @state() private _loadAllPending: boolean = false;

  @state() private _loadingTarget: number = 0;

  // Permanently bypasses chunked loading limits once the full list has been requested or rendered.
  @state() private _paginationDisabled: boolean = false;

  // Caches generated graph elements to prevent redundant network requests and iframe reflows.
  private _graphCache: Map<string, ExploreSimpleSk> = new Map();

  // Tracks which graphs have successfully resolved their initial 'data-loaded' event.
  private _loadedGraphIds: Set<string> = new Set();

  // Registry to record the original sequence of items provided by the parent.
  // Used to restore an item's correct historical position if it is removed and later re-added.
  private _masterOrder: string[] = [];

  protected updated(changedProperties: PropertyValues) {
    super.updated(changedProperties);
    if (changedProperties.has('items')) {
      // Learn and register the order of any new items provided by the parent.
      this.items.forEach((item) => {
        if (!this._masterOrder.includes(item.id)) {
          this._masterOrder.push(item.id);
        }
      });

      this.reconcile();
    }
  }

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      ${this.renderControls()}
      <div
        id="graph-container"
        @x-axis-toggled=${this.syncXAxisLabel}
        @range-changing-in-multi=${this.syncExtendRangeOnSummaryBar}
        @selection-changing-in-multi=${this.syncChartSelection}
        @even-x-axis-spacing-changed=${this.syncEvenXAxisSpacing}></div>
      ${this.renderControls()}
    `;
  }

  private renderControls() {
    return html`
      <div class="controls">
        <div class="status-info">
          ${this._renderedCount > 0 && this._renderedCount < this.items.length
            ? html`<span>Showing ${this._renderedCount} of ${this.items.length} graphs.</span>`
            : ''}
          ${this._currentlyLoading
            ? html`
                <span class="loading-status">
                  <spinner-sk id="loading-spinner" active></spinner-sk>
                  <span class="loading-message">${this._currentlyLoading}</span>
                </span>
              `
            : ''}
        </div>
        ${this._renderedCount < this.items.length &&
        !this._loadAllPending &&
        this._loadingTarget < this.items.length &&
        !this._paginationDisabled
          ? html`
              <div class="load-buttons">
                <button
                  @click=${() => this.loadMore(false)}
                  ?disabled=${this._currentlyLoading !== ''}>
                  Load next ${Math.min(this.visibleLimit, this.items.length - this._renderedCount)}
                </button>
                <button @click=${this.triggerLoadAll}>Load All</button>
              </div>
            `
          : ''}
      </div>
    `;
  }

  private async triggerLoadAll() {
    if (this._currentlyLoading) {
      this._loadAllPending = true;
    } else {
      await this.loadMore(true);
    }
  }

  @query('#graph-container')
  private _graphDiv!: HTMLDivElement;

  public async reload() {
    if (this._graphDiv) {
      this._graphDiv.replaceChildren();
    }
    this._graphCache.clear();
    this._loadedGraphIds.clear();
    this._renderedCount = 0;
    this._allGraphsLoaded = false;
    this._paginationDisabled = false;
    await this.updateComplete;
    await this.loadMore();
  }

  private async reconcile() {
    await this.updateComplete;
    if (!this._graphDiv) return;

    // Prune cached elements that are no longer present in the updated items array.
    const newItemsSet = new Set(this.items.map((i) => i.id));
    for (const [id, el] of this._graphCache) {
      if (!newItemsSet.has(id)) {
        el.remove();
      }
    }

    // Resetting the count ensures that loadMore() iterates through the entire
    // current list from the beginning.
    // Allows sequential appendChild operations to naturally sort the physical DOM.
    this._renderedCount = 0;
    this._allGraphsLoaded = false;

    await this.loadMore();
  }

  public getElementForId(id: string): ExploreSimpleSk | undefined {
    return this._graphCache.get(id);
  }

  public addGraph(item: GraphItem) {
    if (this.items.some((i) => i.id === item.id)) return;

    // Register the item if it has never been seen before
    if (!this._masterOrder.includes(item.id)) {
      this._masterOrder.push(item.id);
    }

    // Sort the updated array based on the historical master order to guarantee original positioning
    const newItems = [...this.items, item];
    newItems.sort((a, b) => this._masterOrder.indexOf(a.id) - this._masterOrder.indexOf(b.id));

    this.items = newItems;
  }

  public removeGraph(id: string) {
    this.items = this.items.filter((item) => item.id !== id);
  }

  public async loadMore(loadAll: boolean = false) {
    if (this._currentlyLoading) return;
    await this.updateComplete;
    if (!this._graphDiv) return;

    if (loadAll) {
      this._paginationDisabled = true;
    }

    const targetCount = this._paginationDisabled
      ? this.items.length
      : Math.min(this._renderedCount + this.visibleLimit, this.items.length);

    if (this._renderedCount >= targetCount) {
      this.dispatchEvent(new CustomEvent('graphs-loaded', { bubbles: true }));
      return;
    }

    this._loadingTarget = targetCount;
    const itemsToLoad = this.items.slice(this._renderedCount, targetCount);
    let loadedCount = 0;
    this._allGraphsLoaded = false;
    const currentItemsRef = this.items;

    for (let i = 0; i < itemsToLoad.length; i += this.chunkSize) {
      // Abort gracefully if the items array is mutated by a concurrent user action.
      if (this.items !== currentItemsRef) return;

      this._currentlyLoading = `Loading (${
        this._renderedCount + loadedCount
      }/${targetCount})... [out of ${this.items.length} total]`;

      const chunk = itemsToLoad.slice(i, i + this.chunkSize);

      chunk.forEach((item) => {
        const id = item.id;
        let graphElement = this._graphCache.get(id) ?? null;

        if (!graphElement) {
          graphElement = item.generateGraph();
          if (graphElement) {
            this._graphCache.set(id, graphElement);
          }
        }

        if (graphElement) {
          // Calling appendChild ensures the node is moved to the end of the container,
          // matching the array's exact sorted order.
          this._graphDiv!.appendChild(graphElement);
          const height = this.items.length === 1 ? '500px' : '250px';
          graphElement.updateChartHeight(height);
        }
      });

      const promises = chunk.map(
        (item) =>
          new Promise<void>((resolve) => {
            const id = item.id;
            const graphElement = this._graphCache.get(id);

            if (!graphElement || this._loadedGraphIds.has(id)) {
              loadedCount++;
              resolve();
              return;
            }

            const listener = () => {
              graphElement.removeEventListener('data-loaded', listener);
              this._loadedGraphIds.add(id);
              loadedCount++;
              resolve();
            };

            graphElement.addEventListener('data-loaded', listener);

            // Failsafe: Releasing the lock if data-loaded is not dispatched,
            // preventing the UI from hanging indefinitely on complex queries.
            setTimeout(() => {
              graphElement.removeEventListener('data-loaded', listener);
              this._loadedGraphIds.add(id);
              resolve();
            }, 60000);
          })
      );

      await Promise.all(promises);
      this.updateChartHeights();
    }

    this._renderedCount = targetCount;
    this._loadingTarget = 0;
    this._currentlyLoading = '';

    // Lock expansion open permanently if all available items have been rendered.
    if (this._renderedCount >= this.items.length) {
      this._allGraphsLoaded = true;
      this._paginationDisabled = true;
    }

    // Process a 'Load All' command if clicked while a chunk was actively resolving.
    if (this._loadAllPending) {
      this._loadAllPending = false;
      await this.loadMore(true);
    }

    this.dispatchEvent(new CustomEvent('graphs-loaded', { bubbles: true }));

    const graphs = this.getGraphs();
    if (graphs.length > 0) {
      graphs[0].broadcastSelection();
    }
  }

  public getGraphs(): ExploreSimpleSk[] {
    if (!this._graphDiv) return [];
    return Array.from(this._graphDiv.querySelectorAll('explore-simple-sk'));
  }

  private updateChartHeights(): void {
    const graphs = this.getGraphs();
    graphs.forEach((graph) => {
      const height = graphs.length === 1 ? '500px' : '250px';
      graph.updateChartHeight(height);
    });
  }

  // --- Synchronization Handlers ---
  private async syncExtendRangeOnSummaryBar(
    e: CustomEvent<PlotSelectionEventDetails>
  ): Promise<void> {
    const graphs = this.getGraphs();
    const offset = e.detail.offsetInSeconds;
    const range = e.detail.value;

    graphs.forEach(async (graph) => {
      await graph.extendRange(range, offset);
    });
  }

  private async syncChartSelection(e: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    const graphs = this.getGraphs();
    if (!e.detail.value) return;

    if (graphs.length > 1 && e.detail.offsetInSeconds !== undefined) {
      await graphs[0].extendRange(e.detail.value, e.detail.offsetInSeconds);
    }

    graphs.forEach((graph, i) => {
      if (i !== e.detail.graphNumber && e.detail.offsetInSeconds === undefined) {
        graph.updateSelectedRangeWithPlotSummary(
          e.detail.value,
          e.detail.start ?? 0,
          e.detail.end ?? 0
        );
      }
    });

    graphs.forEach(async (graph, i) => {
      if (i !== e.detail.graphNumber && e.detail.offsetInSeconds !== undefined) {
        await graph.requestComplete;
        graph.updateSelectedRangeWithPlotSummary(
          e.detail.value,
          e.detail.start ?? 0,
          e.detail.end ?? 0
        );
      }
    });
  }

  private syncXAxisLabel(e: CustomEvent): void {
    const graphs = this.getGraphs();
    graphs.forEach((graph) => graph.switchXAxis(e.detail));
  }

  private syncEvenXAxisSpacing(e: CustomEvent): void {
    const newValue = e.detail.value;
    const graphs = this.getGraphs();
    graphs.forEach((graphNode) => {
      const graph = graphNode as ExploreSimpleSk;
      if (graph.state.graph_index !== e.detail.graph_index) {
        graph.state.evenXAxisSpacing = newValue;
        graph.setUseDiscreteAxis(newValue);
        graph.render();
      }
    });
  }
}

define('graph-list-sk', GraphListSk);
