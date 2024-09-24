/**
 * @module modules/plot-summary-sk/h_resizable_box_sk
 * @description <h2><code>h-resizable-box-sk</code></h2>
 *
 * This is created to support UI interactions for summary bar. This component
 * itself may be used for drawing and dragging a range horizontally.
 * The element will need to cover the entire parent node to work.
 *
 * @evt selection-changed - Emit when the selectionRange is changed.
 *  It is not bubbled, and its details contains the selectionRange.
 *
 */
import { range } from '../dataframe/index';

import { css, html, LitElement } from 'lit';
import { customElement } from 'lit/decorators.js';
import { ref, createRef } from 'lit/directives/ref.js';

// clamps the value in between min and max. This helps to compute
// rect box not exceeding its boundary.
// if the given max is larger than min. val is clamped to max first.
const clamp = (val: number, min: number, max: number) => {
  if (val > max) {
    return max;
  } else if (val < min) {
    return min;
  } else {
    return val;
  }
};

@customElement('h-resizable-box-sk')
export class HResizableBoxSk extends LitElement {
  static styles = css`
    :host {
      display: block;
    }
    .container {
      position: absolute;
      top: 0px;
      left: 0px;
      width: 100%;
      height: 100%;
    }
    .surface {
      display: none;
      position: absolute;
      border-radius: 6px;
      top: 0px;
      bottom: 0px;
      left: 0px;
      right: 0px;
      cursor: move;
      --md-elevation-level: 1;
      z-index: 10;
      background-color: #627eb02f;
    }
    .surface::after {
      content: '';
      position: absolute;
      left: 0px;
      width: 4px;
      height: 100%;
      cursor: ew-resize;
    }
    .surface::before {
      content: '';
      position: absolute;
      right: 0px;
      width: 4px;
      height: 100%;
      cursor: ew-resize;
    }
  `;

  private selection = createRef<HTMLDivElement>();

  // The handle bar width for resizing.
  private handleWidth = 4;

  // The minimum width that allows dragging and resizing.
  private minWidth = 24;

  // This tracks the current user intention:
  // * drag: the user moves the selection box;
  // * left: the user drags the left edge to resize;
  // * right: the user drags the right edge to resize;
  // * draw: the user starts a new selection.
  private action: 'drag' | 'left' | 'right' | 'draw' | null = null;

  private lastX = 0;

  private startX = 0;

  // set and draw the selection box. hidden if null.
  // the range is relative to the element itself.
  set selectionRange(range: range | null) {
    const box = this.selection.value!;
    if (!range) {
      box.style.display = 'none';
      box.style.width = '0px';
      return;
    } else {
      box.style.display = 'block';
    }

    const parentRt = this.getBoundingClientRect();
    box.style.left = clamp(range.begin, 0, parentRt.width) + 'px';
    box.style.width =
      clamp(
        range.end - range.begin,
        this.minWidth,
        parentRt.width - range.begin
      ) + 'px';
  }

  // current selection box range.
  // the range is relative to the element itself.
  get selectionRange() {
    const box = this.selection.value;
    if (!box || box.style.display === 'none') {
      return null;
    }

    const parentRt = this.getBoundingClientRect();
    const rect = box.getBoundingClientRect();
    return {
      begin: rect.left - parentRt.left,
      end: rect.right - parentRt.left,
    } as range;
  }

  private onMouseDown(e: MouseEvent) {
    // If the mouseDown is starting from our component, then we takes it over.
    // This disable system events like selecting texts.
    e.preventDefault();

    const box = this.selection.value!;
    const rect = box.getBoundingClientRect();
    this.lastX = e.x;
    this.startX = e.x;
    if (e.target !== box) {
      this.action = 'draw';
      box.style.display = 'block'; // the selection box can be hidden if no selection.
    } else if (e.x - rect.left < this.handleWidth) {
      this.action = 'left';
    } else if (e.x > rect.right - this.handleWidth) {
      this.action = 'right';
    } else {
      this.action = 'drag';
    }
  }

  private onMouseMove(e: MouseEvent) {
    if (!this.action) {
      return;
    }

    e.preventDefault();
    const box = this.selection.value!;
    const parentRt = this.getBoundingClientRect();
    const rect = box.getBoundingClientRect();
    const delta = this.lastX - e.x;
    const localLeft = rect.left - parentRt.left;
    const localRight = rect.right - parentRt.left;

    // We start to draw a new selection.
    if (this.action === 'draw') {
      // we drag the mouse to the left, and then we draw a box with a minWidth.
      if (e.x < this.startX) {
        const left = clamp(
          e.x - parentRt.left,
          0,
          this.startX - parentRt.left - this.minWidth
        );
        box.style.left = left + 'px';
        box.style.width =
          clamp(this.startX - e.x, this.minWidth, parentRt.width - left) + 'px';
      } else {
        // We drag the mouse to the right.
        const left = clamp(
          this.startX - parentRt.left,
          0,
          parentRt.width - this.minWidth
        );
        box.style.left = left + 'px';
        box.style.width =
          clamp(e.x - this.startX, this.minWidth, parentRt.width - left) + 'px';
      }
    }

    if (this.action === 'drag') {
      // We drag and move the selection box.
      box.style.left =
        clamp(localLeft - delta, 0, parentRt.width - rect.width) + 'px';
      box.style.width = rect.width + 'px';
    } else if (this.action === 'left') {
      // We drag the left edge of the box to resize it.
      const left = clamp(localLeft - delta, 0, localRight - this.minWidth);
      box.style.left = left + 'px';
      box.style.width = localRight - left + 'px';
    } else if (this.action === 'right') {
      // We drag the right edge of the box to resize it.
      const newLeft = rect.left - parentRt.left;
      box.style.left = rect.left - parentRt.left + 'px';
      box.style.width =
        clamp(rect.width - delta, this.minWidth, parentRt.width - newLeft) +
        'px';
    }
    this.lastX = e.x;
  }

  private onMouseUp() {
    if (this.action === null) {
      return;
    }
    this.action = null;
    this.dispatchEvent(
      new CustomEvent('selection-changed', {
        detail: this.selectionRange,
      })
    );
  }

  protected render() {
    return html`<div
      class="container"
      @mousedown=${(e: MouseEvent) => this.onMouseDown(e)}>
      <div class="surface" ${ref(this.selection)}>
        <md-elevation></md-elevation>
      </div>
    </div>`;
  }

  connectedCallback(): void {
    super.connectedCallback();
    // We have mousedown event on the element so we start tracking that's
    // originated from ourselves. The default bounding box check helps us
    // initiate the tracking.
    // We listen to mousemove and moseup on Window so even the mouse is moving
    // outside the element, we can still get the callback.
    window.addEventListener('mousemove', (e) => {
      this.onMouseMove(e);
    });
    window.addEventListener('mouseup', () => {
      this.onMouseUp();
    });
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'h-resizable-box-sk': HResizableBoxSk;
  }
}
