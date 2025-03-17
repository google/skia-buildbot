/**
 * DragToZoomBoxSk is a selection box applied on top of the google chart
 * when a user ctrl click onto it. The user can drag the selection horizontally
 * and vertically and this module will estimate the difference between the
 * values based on the start and end cursor positions.
 */

import { css, html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('drag-to-zoom-box-sk')
export class DragToZoomBox extends LitElement {
  static styles = css`
    :host {
      display: none;
      position: absolute; /* note: this interferes with calculating offsetY */
      border-style: solid;
      border-width: 1px 1px;
      border-color: rgba(255, 99, 71, 0.4); /* almost orange red color */
      background-color: rgba(255, 99, 68, 0.4); /* semi-transparent shade of orange hue */
      --md-elevation-level: 1;
      cursor: magnifying glass;
    }
  `;

  protected render() {
    return html`<md-elevation></md-elevation>`;
  }

  // mark the starting cursor location in the plot-google-chart
  // start represents the selection range of either top, left, bottom or right
  // we need to track the start coordinate because getBoundingClientRect
  // is with respect to the viewport
  @property()
  startPosition = {
    xOffset: 0,
    yOffset: 0,
  };

  // store the chart boundary information in the plot-google-chart
  @property()
  private boundaryInfo = {
    width: 0,
    height: 0,
  };

  /**
   * Initialize pass into helper variables to show the vertical or horizontal selection box.
   * @param chartArea key positions of the google chart. Top and bottom are used to
   * determine the max and min boundary horizontally. Left and right are determined the
   * min and max boundary vertically.
   * Height and width allow the selection to cleanly cover the chart.
   * @param start the coordinate and y value of the chart where the cursor is
   */
  initializeShow(
    chartArea: { top: number; left: number; width: number; height: number },
    start: { xOffset: number; yOffset: number }
  ) {
    this.startPosition = start;

    this.style.top = `${chartArea.top}px`;
    this.style.left = `${chartArea.left}px`;
    this.boundaryInfo.width = chartArea.width;
    this.boundaryInfo.height = chartArea.height;
    this.style.display = 'block';
  }

  /**
   * Hide the module by switching the display to none
   */
  hide() {
    this.style.display = 'none';
    this.style.height = `0px`;
    this.style.width = `0px`;
  }

  /**
   * Handle a drag event and update the selection area relative to the cursor start point
   * @param update the updated cursor coordinate and boolean to distinguish if
   * the direction is horizontal or vertical
   */
  handleDrag(update: { offset: number; isHorizontal: boolean }) {
    if (update.isHorizontal) {
      const newLeft = Math.min(update.offset, this.startPosition.xOffset);
      const width = Math.abs(update.offset - this.startPosition.xOffset);
      this.style.left = `${newLeft}px`;
      this.style.width = `${width}px`;
      this.style.height = `${this.boundaryInfo.height}px`;
    } else {
      const newTop = Math.min(update.offset, this.startPosition.yOffset);
      const height = Math.abs(update.offset - this.startPosition.yOffset);
      this.style.top = `${newTop}px`;
      this.style.height = `${height}px`;
      this.style.width = `${this.boundaryInfo.width}px`;
    }
  }
}
