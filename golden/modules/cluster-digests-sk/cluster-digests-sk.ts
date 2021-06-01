/**
 * @module module/cluster-digests-sk
 * @description <h2><code>cluster-digests-sk</code></h2>
 *
 * This element renders a list of digests in a D3 force layout. That is, it draws them as circles
 * (aka nodes) as if they were attached via springs that are proportional to the difference between
 * the digest images; the nodes repel each other, as if they were charged particles.
 *
 * It is strongly recommended to have the d3 documentation handy, as this element makes heavy use
 * of that (somewhat dense) library.
 *
 * TODO(kjlubick) make this interactive, like the old Polymer element was.
 *
 * @evt layout-complete; fired when the force layout has stabilized (i.e. finished rendering).
 *
 * @evt selection-changed; fired when a digest is clicked on (or the selection is cleared). Detail
 *   contains a list of digests that are selected.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import * as d3Force from 'd3-force';
import * as d3Select from 'd3-selection';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {ClusterDiffLink, ClusterDiffNode, Digest} from '../rpc_types';
import {SimulationLinkDatum, SimulationNodeDatum} from 'd3-force';

/**
 * A node returned by the Gold backend, with an optional label added by client-side code.
 *
 * If present, the label will be rendered in the cluster view, next to the node circle.
 */
export interface ClusterDiffNodeWithLabel extends ClusterDiffNode {
  label?: string;
}

type SimNode = ClusterDiffNodeWithLabel & SimulationNodeDatum;
type SimLink = ClusterDiffLink & SimulationLinkDatum<SimNode>;

export class ClusterDigestsSk extends ElementSk {
  private static template = (ele: ClusterDigestsSk) => html`
    <svg width=${ele.width} height=${ele.height}></svg>
  `;

  private nodes: SimNode[] = [];
  private links: SimLink[] = [];

  private linkTightness = 1 / 8;
  private nodeRepulsion = 256;

  private width = 400;
  private height = 400;

  // An array of digests (strings) that correspond to the currently selected digests (if any).
  private selectedDigests: Digest[] = [];

  constructor() {
    super(ClusterDigestsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  changeLinkTightness(isScaleUp: boolean) {
    if (isScaleUp) {
      this.linkTightness *= 1.5;
    } else {
      this.linkTightness /= 1.5;
    }
    this.layout();
  }

  changeNodeRepulsion(isScaleUp: boolean) {
    if (isScaleUp) {
      this.nodeRepulsion *= 1.5;
    } else {
      this.nodeRepulsion /= 1.5;
    }
    this.layout();
  }

  /**
   * Recomputes the positions of the digest nodes given the value of links. It expects all the SVG
   * elements (e.g. circles, lines) to already be created; this function will simply update the
   * X and Y values accordingly.
   */
  private layout() {
    const clusterSk = this.querySelector('svg')!;

    // This force acts as a repulsion force between digest nodes. This acts a lot like charged
    // particles repelling one another. The main purpose here is to keep nodes from overlapping.
    // See https://github.com/d3/d3-force#forceManyBody
    const chargeForce = d3Force.forceManyBody()
      .strength(-this.nodeRepulsion)
      // Given our nodes have a radius of 12, if two nodes are 60 pixels apart, they are definitely
      // not overlapping, so we can stop counting their "charge". This should help performance by
      // reducing computation needs.
      .distanceMax(60);

    // This force acts as a spring force between digest nodes. More similar digests pull more
    // tightly and should be closer together.
    // See https://github.com/d3/d3-force#links
    const linkForce = d3Force.forceLink(this.links)
      .distance((d) => d.value / this.linkTightness);

    // This force keeps the diagram centered in the SVG.
    // See https://github.com/d3/d3-force#centering
    const centerForce = d3Force.forceCenter(this.width / 2, this.height / 2);

    // These forces help keep the nodes in the visible area.
    const xForce = d3Force.forceX(this.width / 2);
    xForce.strength(0.1);
    const yForce = d3Force.forceY(this.height / 2);
    yForce.strength(0.2); // slightly stronger force down since we have more width to draw into

    // This starts a simulation that will render over the next few seconds as the nodes are
    // simulated into place.
    // See https://github.com/d3/d3-force#forceSimulation
    d3Force.forceSimulation(this.nodes)
      .force('charge', chargeForce) // The names are arbitrary (and inspired by D3 documentation).
      .force('link', linkForce)
      .force('center', centerForce)
      .force('fitX', xForce)
      .force('fixY', yForce)
      .alphaDecay(0.03395) // 1 - pow(0.001, 1 / 200); i.e. 200 iterations
      .on('tick', () => {
        // On each tick, the simulation will update the x,y values of the nodes. We can then
        // select and update those nodes.
        d3Select.select(clusterSk)
          .selectAll<SVGCircleElement, SimNode>('.node')
          .attr('cx', (d) => d.x!)
          .attr('cy', (d) => d.y!);

        d3Select.select(clusterSk)
          .selectAll<SVGTextElement, SimNode>('.label')
          .attr('x', (d) => d.x! + 14) // offset the labels from the center of the nodes.
          .attr('y', (d) => d.y! + 20);

        // Type guard to narrow down the type of the source and target fields in a SimLink. See
        // https://github.com/DefinitelyTyped/DefinitelyTyped/blob/9bd58256d08405c4e0ee3b065efc984f9a28ca17/types/d3-force/d3-force-tests.ts#L647.
        function isSimNode(maybeNode: SimNode | string | number): maybeNode is SimNode {
          return typeof maybeNode !== 'string' && typeof maybeNode !== 'number';
        }

        // source and target are supplied and updated by forceLink:
        // https://github.com/d3/d3-force#link_links
        d3Select.select(clusterSk)
          .selectAll<SVGLineElement, SimLink>('.link')
          .attr('x1', (d) => isSimNode(d.source) ? d.source.x! : 0)
          .attr('y1', (d) => isSimNode(d.source) ? d.source.y! : 0)
          .attr('x2', (d) => isSimNode(d.target) ? d.target.x! : 0)
          .attr('y2', (d) => isSimNode(d.target) ? d.target.y! : 0);
      })
      .on('end', () => {
        this.dispatchEvent(new CustomEvent('layout-complete', { bubbles: true }));
      });
  }

  private getNodeCSSClass(d: SimNode) {
    let base = `node ${d.status}`;
    if (this.selectedDigests.includes(d.name)) {
      base += ' selected';
    }
    return base;
  }

  /** Sets the new data to render in a cluster view. */
  setData(nodes: ClusterDiffNodeWithLabel[], links: ClusterDiffLink[]) {
    this.nodes = nodes;
    this.links = links;

    this._render();
    // For reasons unknown, after render, we don't always see the SVG element rendered in our
    // DOM, so we schedule the drawing for the next animation frame (when we *do* see the SVG
    // in the DOM).
    window.requestAnimationFrame(() => {
      const clusterSk = this.querySelector('svg')!;

      // Delete existing SVG elements
      d3Select.select(clusterSk)
        .selectAll('.link,.node,.label')
        .remove();

      // Reset selection.
      this.selectedDigests = [];

      // We don't have any lines or dots spawn in or dynamically get removed from the drawing, so
      // we don't need to supply an id function to the data calls below.

      // Draw the lines first so they are behind the circles.
      d3Select.select(clusterSk)
        .selectAll('line.link')
        .data(this.links)
        .enter()
        .append('line')
        .attr('class', 'link')
        .attr('stroke', '#ccc')
        .attr('stroke-width', '2');

      // Draw the labels behind the circles because the circles are clickable.
      d3Select.select(clusterSk)
        .selectAll('text.label')
        .data(this.nodes)
        .enter()
        .append('text')
        .attr('class', 'label');
      d3Select.select(clusterSk) // update all nodes with the correct label.
        .selectAll<SVGTextElement, SimNode>('text.label')
        .text((d) => d.label || '');

      // Draw a circle for each node.
      d3Select.select(clusterSk)
        .selectAll('circle.node')
        .data(this.nodes)
        .enter()
        .append('circle')
        .attr('class', (d) => this.getNodeCSSClass(d))
        .attr('r', 12)
        .attr('stroke', 'black')
        .attr('data-digest', (d) => d.name)
        .on('click tap', (d) => {
          // Capture this event (prevent it from propagating to the SVG).
          const evt = d3Select.event;
          evt.preventDefault();
          evt.stopPropagation();

          const digest = d.name;
          if (this.selectedDigests.includes(digest)) {
            return; // It's already selected, do nothing.
          }
          if (evt.shiftKey || evt.ctrlKey || evt.metaKey) {
            // Support multiselection if shift, control or meta is held.
            this.selectedDigests.push(digest);
          } else {
            // Clear the existing selection and replace it with this digest.
            this.selectedDigests = [digest];
          }
          this.updateSelection();
        });

      d3Select.select(clusterSk).on('click tap', () => {
        // Capture this event (prevent it from propagating outside the SVG).
        const evt = d3Select.event;
        evt.preventDefault();
        evt.stopPropagation();

        this.selectedDigests = [];
        this.updateSelection();
      });

      this.layout();
    });
  }

  setWidth(w: number) {
    if (w === this.width) {
      // Don't need to re-render if the width is unchanged.
      return;
    }
    this.width = w;
    this.layout();
  }

  private updateSelection() {
    d3Select.select(this.querySelector('svg'))
      .selectAll('circle.node')
      .data(this.nodes)
      .attr('class', (d) => this.getNodeCSSClass(d));

    this.dispatchEvent(new CustomEvent<Digest[]>('selection-changed', {
      bubbles: true,
      detail: this.selectedDigests,
    }));
  }
}

define('cluster-digests-sk', ClusterDigestsSk);
