/**
 * VResizeableBoxSk is a selection box applied on top of the google chart
 * when a user shift-clicks onto it. The user can drag the selection up
 * and/or down and this module will estimate the difference between the
 * values based on the start and end cursor positions.
 */
import { css, html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';

const fontPosition = {
  upper: -35,
  lower: -10,
};

@customElement('v-resizable-box-sk')
export class VResizableBoxSk extends LitElement {
  // TODO(b/361365957): create a light/dark mode theme color + opacity
  // and apply variables to the border-color and background-color
  static styles = css`
    :host {
      display: none;
      position: absolute;
      border-style: solid;
      border-width: 1px 0px; /* no borders left and right */
      border-color: rgba(255, 255, 0, 0.5); /* yellow */
      background-color: rgba(255, 255, 0, 0.2); /* almost transparent yellow */
      this.style.height: 0px;
      --md-elevation-level: 1;
      cursor: row-resize;
    }

    p {
      position: absolute;
      left: 30px;
      font-size: 16px;
      text-shadow:
        -1px -1px 0 var(--md-sys-color-background, 'white'),
        1px -1px 0 var(--md-sys-color-background, 'white'),
        -1px 1px 0 var(--md-sys-color-background, 'white'),
        1px 1px 0 var(--md-sys-color-background, 'white');
    }
    /* show the delta calculation slightly above the box */
    p.upper {
      top: ${fontPosition.upper}px;
    }
    /* show the delta calculation slightly below the box */
    p.lower {
      top: ${fontPosition.lower}px;
    }
  `;

  protected render() {
    return html`
      <p class=${this.lowerFont ? 'lower' : 'upper'}>${this.delta.raw} or ${this.delta.percent}%</p>
      <md-elevation></md-elevation>
    `;
  }

  // the starting cursor location and value of the box relative
  // to the parent plot-google-chart
  // start represents either the top or bottom of the selection range
  // we need to track the start coordinate because getBoundingClientRect
  // is with respect to the viewport
  @property()
  private start = {
    coord: 0,
    value: 0,
  };

  // stores the selection range delta in units of the traces as a string
  // rounded to 4 significant digits (for readability)
  @property()
  private delta = {
    raw: '',
    percent: '',
  };

  // if the selection is too tall, then the delta font will break out
  // of the google chart and conflict with other visual elements
  // when that happens, lower the font
  @property()
  private lowerFont = false;

  // the top boundary of the chart. Used to calculate if the selection
  // range is too close to the top boundary.
  private topBoundary: null | number = null;

  /**
   * Initialize helper variables and show the vertical selection.
   * By default, show the selection as the same width as the google
   * chart area.
   * @param chartArea key positions of the google chart. Top is used to
   * determine if the delta font should be above or below the selection.
   * Left and width allow the selection to cleanly cover the chart.
   * @param start the coordinate and y value of the chart where the cursor is
   */
  show(
    chartArea: { top: number; left: number; width: number },
    start: { coord: number; value: number }
  ) {
    this.start = start;
    this.updateDelta(start.value);

    this.topBoundary = chartArea.top;
    this.lowerFont = start.coord + fontPosition.upper < this.topBoundary!;

    this.style.top = `${start.coord}px`;
    this.style.left = `${chartArea.left}px`;
    this.style.width = `${chartArea.width}px`;

    this.style.display = 'block';
  }

  /**
   * Hide the module by switching the display to none
   */
  hide() {
    this.style.display = 'none';
    this.style.height = `0px`;
  }

  /**
   * Update the height and deltas as the user moves the selection up and/or down
   * @param update the updated cursor coordinate and respective y-axis chart value
   */
  updateSelection(update: { coord: number; value: number }) {
    this.updateDelta(update.value);
    this.updateBox(update.coord);

    // lower the font if the selection's upper boundary is too close to the top
    // boundary of the graph
    const selectionTopCoord = Math.min(this.start.coord, update.coord);
    this.lowerFont = selectionTopCoord + fontPosition.upper < this.topBoundary!;
  }

  // update the height of the box relative to the cursor start point
  private updateBox(newY: number) {
    const newTop = Math.min(newY, this.start.coord);
    const height = Math.abs(newY - this.start.coord);
    this.style.top = `${newTop}px`;
    this.style.height = `${height}px`;
  }

  // update the deltas relative to the selection starting value
  private updateDelta(b: number) {
    const a = this.start.value;
    this.delta = {
      // only display 4 significant digits for readability
      raw: (b - a).toPrecision(4),
      // show NaN if start value is exactly 0
      percent: a === 0 ? 'NaN' : (((b - a) / a) * 100).toPrecision(4),
    };
  }
}
