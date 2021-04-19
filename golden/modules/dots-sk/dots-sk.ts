/**
 * @module modules/dots-sk
 * @description <h2><code>dots-sk</code></h2>
 *
 * A custom element for displaying a dot chart of digests by trace, such as:
 *
 *   ooo-o-o-oo
 *
 * @evt showblamelist - Event generated when a dot is clicked. e.detail contains
 *   the blamelist (an array of commits that could have made up that dot).
 *
 * @evt hover - Event generated when the mouse hovers over a trace. e.detail is
 *   the trace id.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  dotToCanvasX,
  dotToCanvasY,
  DOT_FILL_COLORS,
  DOT_FILL_COLORS_HIGHLIGHTED,
  DOT_OFFSET_X,
  DOT_OFFSET_Y,
  DOT_RADIUS,
  DOT_SCALE_X,
  DOT_SCALE_Y,
  DOT_STROKE_COLORS,
  MISSING_DOT,
  STROKE_WIDTH,
  TRACE_LINE_COLOR,
} from './constants';
import { Commit, Trace, TraceGroup } from '../rpc_types';

// Array of dots-sk component instances. A dots-sk instance is present if it has a pending
// mousemove update.
const dotsSkInstancesWithPendingMouseMoveUpdates: DotsSk[] = [];

// Periodically process all pending mousemoves. We do not want to do any work on a mouse move event
// as that can very easily degrade browser performance, e.g. as the user drags the mouse pointer
// over the element. Processing mouse events in batches remedies this.
setInterval(() => {
  while (dotsSkInstancesWithPendingMouseMoveUpdates.length > 0) {
    const dotsSk = dotsSkInstancesWithPendingMouseMoveUpdates.pop()!;
    dotsSk.updatePendingMouseMove();
  }
}, 40);

export class DotsSk extends ElementSk {
  private static template = () => html`<canvas></canvas>`;

  private canvas: HTMLCanvasElement | null = null;
  private ctx: CanvasRenderingContext2D | null = null;

  private _commits: Commit[] = [];
  private _value: TraceGroup = {tileSize: 0, traces: [], digests: [], total_digests: 0};

  // The index of the trace that should be highlighted.
  private hoverIndex = -1;
  private hasScrolledOnce = false;

  // For capturing the last mousemove event, which is later processed in a timer.
  private lastMouseMove: MouseEvent | null = null;

  constructor() {
    super(DotsSk.template);

    // Explicitly bind event handler methods to this.
    this.onMouseMove = this.onMouseMove.bind(this);
    this.onMouseLeave = this.onMouseLeave.bind(this);
    this.onClick = this.onClick.bind(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.canvas = $$('canvas', this);
    this.canvas!.addEventListener('mousemove', this.onMouseMove);
    this.canvas!.addEventListener('mouseleave', this.onMouseLeave);
    this.canvas!.addEventListener('click', this.onClick);
    this.ctx = this.canvas!.getContext('2d');
    this.draw();
  }

  disconnectedCallback() {
    this.canvas!.removeEventListener('mousemove', this.onMouseMove);
    this.canvas!.removeEventListener('mouseleave', this.onMouseLeave);
    this.canvas!.removeEventListener('click', this.onClick);
    this.hasScrolledOnce = false;
  }

  /**
   * The TraceGroup to display, e.g.:
   *
   *   {
   *     tileSize: 50,
   *     traces: [
   *       {
   *         label: "some:trace:id",
   *         data: [0, 1, 2, 2, 1, -1, -1, 2, ...],
   *       },
   *       ...
   *     ]
   *   }
   *
   * Where the content of the data array are color codes; 0 is the target digest, while 1-6
   * indicate unique digests that are different from the target digest. A code of -1 means no data
   * for the corresponding commit. A code of 7 means that there are 8 or more unique digests in the
   * trace and all digests after the first 8 unique digests are represented by this code. The
   * highest index of data is the most recent data point.
   */
  get value(): TraceGroup { return this._value; }

  set value(value: TraceGroup) {
    if (!value || (value.tileSize === 0)) {
      return;
    }
    this._value = value;
    if (this._connected) {
      this.draw();
    }
  }

  /** An array of Commits associated with the TraceGroup to display. */
  get commits(): Commit[] { return this._commits; }

  set commits(commits: Commit[]) { this._commits = commits; }

  /**
   * Scrolls the traces all the way to the right, showing the newest first. It will only do this
   * on the first call, so as to avoid undoing the user manually scrolling left to see older
   * history.
   */
  autoscroll() {
    if (!this.hasScrolledOnce) {
      this.hasScrolledOnce = true;
      this.scroll(this.scrollWidth, 0);
    }
  }

  /** Draws the entire canvas. */
  private draw() {
    const w = (this._value.tileSize - 1) * DOT_SCALE_X + 2 * DOT_OFFSET_X;
    const h = (this._value.traces!.length - 1) * DOT_SCALE_Y + 2 * DOT_OFFSET_Y;
    this.canvas!.setAttribute('width', `${w}px`);
    this.canvas!.setAttribute('height', `${h}px`);

    // First clear the canvas.
    this.ctx!.lineWidth = STROKE_WIDTH;
    this.ctx!.fillStyle = '#FFFFFF';
    this.ctx!.fillRect(0, 0, w, h);

    // Draw lines and dots.
    this._value.traces!.forEach((trace, traceIndex) => {
      this.ctx!.strokeStyle = TRACE_LINE_COLOR;
      this.ctx!.beginPath();
      const firstNonMissingDot = trace.data!.findIndex((dot) => dot !== MISSING_DOT);
      let lastNonMissingDot = -1;
      for (let i = trace.data!.length - 1; i >= 0; i--) {
        if (trace.data![i] !== MISSING_DOT) {
          lastNonMissingDot = i;
          break;
        }
      }
      if (firstNonMissingDot < 0 || lastNonMissingDot < 0) {
        // Trace was all missing data, so nothing to draw. This should never happen, such a trace
        // would not be included in search results.
        console.warn(`trace with id ${trace.label} was unexpectedly empty`);
        return;
      }
      this.ctx!.moveTo(
        dotToCanvasX(firstNonMissingDot),
        dotToCanvasY(traceIndex),
      );
      this.ctx!.lineTo(
        dotToCanvasX(lastNonMissingDot),
        dotToCanvasY(traceIndex),
      );
      this.ctx!.stroke();
      this.drawTraceDots(trace.data!, traceIndex);
    });
  }

  /** Draws the circles for a single trace. */
  private drawTraceDots(colors: number[], y: number) {
    colors.forEach((c, x) => {
      // We don't draw a dot when it is missing.
      if (c === MISSING_DOT) {
        return;
      }
      this.ctx!.beginPath();
      this.ctx!.strokeStyle = this.getColorSafe(DOT_STROKE_COLORS, c);
      this.ctx!.fillStyle = (this.hoverIndex === y)
        ? this.getColorSafe(DOT_FILL_COLORS_HIGHLIGHTED, c)
        : this.getColorSafe(DOT_FILL_COLORS, c);
      this.ctx!.arc(
        dotToCanvasX(x), dotToCanvasY(y), DOT_RADIUS, 0, Math.PI * 2,
      );
      this.ctx!.fill();
      this.ctx!.stroke();
    });
  }

  /**
   * Used to index into the dot color arrays (DOT_STROKE_COLORS, etc.). Returns
   * the last color in the array if the given unique digest index exceeds
   * MAX_UNIQUE_DIGESTS.
   *
   * This assumes that the color array is of length MAX_UNIQUE_DIGESTS + 1.
   */
  private getColorSafe(colorArray: string[], uniqueDigestIndex: number): string {
    return colorArray[Math.min(colorArray.length - 1, uniqueDigestIndex)];
  }

  /** Redraws just the circles for a single trace. */
  private redrawTraceDots(traceIndex: number) {
    const trace = this._value.traces![traceIndex];
    if (!trace) {
      return;
    }
    this.drawTraceDots(trace.data!, traceIndex);
  }

  private onMouseLeave() {
    const oldHoverIndex = this.hoverIndex;
    this.hoverIndex = -1;
    this.redrawTraceDots(oldHoverIndex);
    this.lastMouseMove = null;
  }

  private onMouseMove(e: MouseEvent) {
    this.lastMouseMove = e;
    dotsSkInstancesWithPendingMouseMoveUpdates.push(this);
  }

  /** Gets the coordinates of the mouse event in dot coordinates. */
  private mouseEventToDotSpace(e: MouseEvent) {
    const rect = this.canvas!.getBoundingClientRect();
    const x = (e.clientX - rect.left - DOT_OFFSET_X + STROKE_WIDTH + DOT_RADIUS)
            / DOT_SCALE_X;
    const y = (e.clientY - rect.top - DOT_OFFSET_Y + STROKE_WIDTH + DOT_RADIUS)
            / DOT_SCALE_Y;
    return { x: Math.floor(x), y: Math.floor(y) };
  }

  /**
   * We look at the mousemove event, if one occurred, to determine which trace to highlight.
   *
   * Not part of the public API.
   */
  updatePendingMouseMove() {
    if (!this.lastMouseMove) {
      return;
    }
    const dotCoords = this.mouseEventToDotSpace(this.lastMouseMove);
    this.lastMouseMove = null;
    // If the focus has moved to a different trace then draw the two changing
    // traces.
    if (this.hoverIndex !== dotCoords.y) {
      const oldIndex = this.hoverIndex;
      this.hoverIndex = dotCoords.y;
      if (this.hoverIndex >= 0
          && this.hoverIndex < this._value.traces!.length) {
        this.dispatchEvent(new CustomEvent('hover', {
          bubbles: true,
          detail: this._value.traces![this.hoverIndex].label,
        }));
      }
      // Just update the dots of the traces that have changed.
      this.redrawTraceDots(oldIndex);
      this.redrawTraceDots(this.hoverIndex);
    }

    // Set the cursor to a pointer if you are hovering over a dot.
    let found = false;
    const trace = this._value.traces![dotCoords.y];
    if (trace) {
      for (let i = trace.data!.length - 1; i >= 0; i--) {
        const dot = trace.data![dotCoords.x]
        if (dot !== undefined && dot !== MISSING_DOT ) {
          found = true;
          break;
        }
      }
    }
    this.style.cursor = (found) ? 'pointer' : 'auto';
  }

  /**
   * When a dot is clicked on, produce the showblamelist event with the blamelist; that is, all the
   * commits that are included up to and including  that dot.
   */
  private onClick(e: MouseEvent) {
    const dotCoords = this.mouseEventToDotSpace(e);
    const trace = this._value.traces![dotCoords.y];
    if (!trace) {
      return; // Misclick, likely.
    }
    const blamelist = this.computeBlamelist(trace, dotCoords.x);
    if (!blamelist) {
      return; // No blamelist if there's no dot at that X coord, i.e. misclick.
    }
    this.dispatchEvent(new CustomEvent('showblamelist', {
      bubbles: true,
      detail: blamelist,
    }));
  }

  /**
   * Takes a trace and the X coordinate of a dot in that trace and returns the blamelist for that
   * dot. The blamelist includes the commit corresponding to the dot, and if the dot is preceded by
   * any missing dots, then their corresponding commits will be included as well.
   */
  private computeBlamelist(trace: Trace, x: number) {
    if (trace.data![x] === MISSING_DOT) {
      // Can happen if there's no dot at that X coord, e.g. misclick.
      return null;
    }
    // Look backwards in the trace for the previous commit with data. If none,
    // 0 is a fine index to compute the blamelist from.
    let lastNonMissingIndex = 0;
    for (let i = x - 1; i >= 0; i--) {
      if (trace.data![i] !== MISSING_DOT) {
        // We include the last non-missing data in our slice because the slice of commits that
        // Gold returns is not the complete history - Gold elides commits that have no data.
        // This is potentially a problem in the following scenario:
        //   - commit 1 has correct data
        //   - commit 2 has correct data
        //   - commit 3 has no data, but introduced a bug (and would have produced incorrect data)
        //   - commit 4 has incorrect data
        // In this case, we need to make sure we can create a blamelist that starts on the first
        // real commit after commit 2. Therefore, we include commit 2 in the list, which GitHub
        // and googlesource will automatically elide when we ask for a range of history (in
        // blamelist-panel-sk). If there were no preceding missing digests, this will equal x-1.
        lastNonMissingIndex = i;
        break;
      }
    }
    const blamelist = this._commits.slice(lastNonMissingIndex, x + 1);
    blamelist.reverse();
    return blamelist;
  }
}

define('dots-sk', DotsSk);
