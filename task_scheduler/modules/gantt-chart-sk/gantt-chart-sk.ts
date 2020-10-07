/**
 * @module gantt
 * @description Tools for displaying Gantt charts.
 */

import { strDuration } from 'common-sk/modules/human';
import { render, svg } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

export interface Block {
  start: Date;
  end: Date;
  color?: string;
  label?: string;
}

export interface Lane {
  label: string;
  blocks: Block[];
}

export interface Data {
  lanes: Lane[];
  start?: Date;
  end?: Date;
  epochs?: Date[];
}

interface DisplayBlock {
  x: number;
  y: number;
  width: number;
  height: number;
  title: string,
  color: string,
}

interface Label {
  text: string;
  x: number;
  y: number;
  width: number;
}

interface Epoch {
  x: number;
  y: number;
  width: number;
  height: number;
  color: string;
}

interface RulerTick {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

interface RulerText {
  x: number;
  y: number;
  rotationX: number;
  rotationY: number;
  text: string;
}

interface Border {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

const blockHeightProportion = 0.8;
const chartMarginLeft = 5;
const chartMarginRight = 110;
const chartMarginY = 5;
const labelFontFamily = 'Arial';
const labelFontSize = 11;
const labelHeight = 20;
const labelMarginRight = 10;
const mouseoverHeight = 50;
const rulerHeight = 78;
const rulerLabelMarginRight = 5;
const rulerLabelRotation = -30;
const rulerTickLength = 15;
const snapThreshold = 15;

/**
 * Draw a chart as a child of the given HTMLElement.
 */
export function draw(container: HTMLElement, data: Data) {
  // Calculate the space needed to display the labels.
  const boundingRect = container.getBoundingClientRect();
  const totalWidth = boundingRect.width;
  const totalHeight = boundingRect.height;
  const blocksHeight = totalHeight - rulerHeight - mouseoverHeight - 2*chartMarginY;
  const rowHeight = blocksHeight / data.lanes.length;
  const blockHeight = rowHeight * blockHeightProportion;
  const blockMarginY = (rowHeight - blockHeight) / 2;
  const labelMarginY = (rowHeight - labelHeight) / 2;

  // This is a temporary throwaway canvas which is only used for measuring
  // text.
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;
  ctx.font = labelFontSize + 'px ' + labelFontFamily;
  let maxLabelWidth = 0;
  data.lanes.forEach((lane: Lane) => {
    let width = ctx.measureText(lane.label).width;
    if (width > maxLabelWidth) {
      maxLabelWidth = width;
    }
  });

  // Define the chart area.
  const blocksStartX = chartMarginLeft + maxLabelWidth + labelMarginRight;
  const blocksStartY = chartMarginY + mouseoverHeight;
  const blocksWidth = totalWidth - blocksStartX - chartMarginRight;

  // Find the time range which encompasses all blocks.
  let tStart = data.start?.getTime() || Number.MAX_SAFE_INTEGER;
  let tEnd = data.end?.getTime() || 0;
  data.lanes.forEach((lane: Lane) => {
    lane.blocks.forEach((block: Block) => {
      const start = new Date(block.start).getTime();
      if (start < tStart) {
        tStart = start;
      }
      if (start > tEnd) {
        tEnd = start;
      }
      const end = new Date(block.end).getTime();
      if (end < tStart) {
        tStart = end;
      }
      if (end > tEnd) {
        tEnd = end;
      }
    });
  });
  if (tStart > tEnd) {
    throw `Start timestamp (${tStart}) is after end (${tEnd})`;
  }
  const duration = tEnd - tStart;
  const timeStart = tStart;

  // Derive the coordinates for the blocks and their labels.
  const blocks: DisplayBlock[] = [];
  const labels: Label[] = [];
  data.lanes.forEach((lane: Lane, laneIndex: number) => {
    labels.push({
      text: lane.label,
      x: chartMarginLeft + maxLabelWidth,
      y: blocksStartY + laneIndex * rowHeight + labelHeight / 2 + labelMarginY,
      width: maxLabelWidth,
    });
    lane.blocks.forEach((block: Block) => {
      const start = block.start.getTime();
      const end = block.end.getTime();
      let title = "";
      if (block.label) {
        title = block.label + " ";
      }
      title += strDuration((end - start) / 1000);
      blocks.push({
        x: blocksStartX + blocksWidth * (start - tStart) / duration,
        y: blocksStartY + laneIndex * rowHeight + blockMarginY,
        width: Math.max(blocksWidth * (end - start) / duration, 1.0),
        height: blockHeight,
        title: title,
        color: block.color || "#000000",
      });
    });
  });

  // Create the ruler.
  // We want approximately one tick every 50-100 px.
  const numTargetTicks = blocksWidth / 75;
  const approxTickSize = duration / numTargetTicks;
  // Round the tick size to the nearest multiple of an appropriate unit.
  // Timestamps are in milliseconds.
  const units = [
           1, //   1 ms
           5, //   5 ms
          10, //  10 ms
          50, //  50 ms
         100, // 100 ms
         500, // 500 ms
        1000, //   1 s
        5000, //   5 s
       10000, //  10 s
       30000, //  30 s
       60000, //   1 m
      300000, //   5 m
      600000, //  10 m
     1800000, //  30 m
     3600000, //   1 h
    10800000, //   3 h
    21600000, //   6 h
    43200000, //  12 h
    86400000, //   1 d
  ];
  let lowestDist = -1;
  let actualTickSize = units[0];
  for (const unit of units) {
    const dist = Math.abs(approxTickSize - unit);
    if (lowestDist === -1 || dist < lowestDist) {
      lowestDist = dist;
      actualTickSize = unit;
    }
  }
  // Find an "anchor" for the ticks to start. We want the ticks to be on a
  // nice round hour/minute/second/millisecond, so take the start timestamp
  // and truncate to the day.
  const tickAnchor = new Date(tStart);
  tickAnchor.setHours(0);
  tickAnchor.setMinutes(0);
  tickAnchor.setSeconds(0);
  tickAnchor.setMilliseconds(0);
  // Create the ticks. The first tick is the first multiple of the tick size
  // which comes after tStart.
  const numTicksPastAnchor = Math.ceil((tStart - tickAnchor.getTime()) / actualTickSize);
  let tick = tickAnchor.getTime() + numTicksPastAnchor * actualTickSize;
  const ticks = [];
  while (tick < tEnd) {
    ticks.push(tick);
    tick += actualTickSize;
  }

  // Create background blocks for each epoch.
  const epochs = data.epochs || [];
  // Ensure that there's an epoch block which reaches to the end of the chart.
  epochs.push(new Date(tEnd));
  const normEpochs: Epoch[] = [];
  const epochColors = [
    '#EFEFEF',
    '#EAEAEA',
  ];
  let lastX = blocksStartX;
  for (let i = 0; i < epochs.length; i++) {
    const epoch = epochs[i].getTime();
    if (epoch >= tStart && epoch <= tEnd) {
      const x = blocksStartX + blocksWidth * (epoch - tStart) / duration;
      normEpochs.push({
        x: lastX,
        y: blocksStartY,
        width: x - lastX,
        height: blocksHeight,
        color: epochColors[i % epochColors.length],
      });
      lastX = x;
    }
  }

  const rulerTicks: RulerTick[] = [];
  const rulerTexts: RulerText[] = [];
  let lastDate = new Date(ticks[0]).getDate();
  for (const tick of ticks) {
    const x = blocksStartX + blocksWidth * (tick - tStart) / duration;
    const y1 = blocksStartY + blocksHeight;
    const y2 = y1 + rulerTickLength;
    rulerTicks.push({
      x1: x,
      y1: y1,
      x2: x,
      y2: y2,
    });
    const d = new Date(tick);
    rulerTexts.push({
      x: x - rulerLabelMarginRight,
      y: y2,
      rotationX: x,
      rotationY: y2,
      text: d.getDate() === lastDate ? d.toLocaleTimeString() : d.toLocaleString(),
    });
    lastDate = d.getDate();
  }

  // Draw border lines around the chart.
  const borders = [
    {
      x1: blocksStartX,
      y1: blocksStartY,
      x2: blocksStartX,
      y2: blocksStartY + blocksHeight,
    },
    {
      x1: blocksStartX,
      y1: blocksStartY + blocksHeight,
      x2: blocksStartX + blocksWidth,
      y2: blocksStartY + blocksHeight,
    },
  ];

  // Event handlers.
  let dragStartX: number | undefined = undefined;

  // Helper function for finding the x-value and timestamp given a mouse
  // event.
  const getMouseX = (e: MouseEvent): number => {
    // Convert event x-coordinate to a coordinate within the chart area.
    let x = e.clientX - boundingRect!.x;
    if (x < blocksStartX) {
      x = blocksStartX;
    } else if (x > boundingRect!.width - chartMarginRight) {
      x = boundingRect!.width - chartMarginRight;
    }
    // Find the nearest block border; if we're close enough, snap the line.
    let nearest = 0;
    let nearestDist = blocksWidth;
    for (const block of blocks) {
      let dist = Math.abs(block.x - x);
      if (dist < nearestDist) {
        nearest = block.x;
        nearestDist = dist;
      }
      dist = Math.abs(block.x + block.width - x);
      if (dist < nearestDist) {
        nearest = block.x + block.width;
        nearestDist = dist;
      }
    }
    if (nearestDist < snapThreshold) {
      x = nearest;
    }
    return x;
  }

  // Helper function for finding the timestamp associated with the given
  // x-coordinate on the chart.
  const getTimeAtMouseX = (x: number): Date => {
    return new Date(timeStart + ((x - blocksStartX) / blocksWidth) * duration);
  };

  // Create a vertical line used on mouseover. This is a helper function used
  // by the mousemove callback function.
  const mouseMoved = (e: MouseEvent) => {
    const svg = $$<SVGElement>("svg", container);
    if (!svg) {
      return;
    }

    // Update the vertical cursor line.
    const x = getMouseX(e);
    const mouseLine = $$<SVGLineElement>("#mouse-line", svg)!;
    mouseLine.setAttributeNS(null, "x1", ""+x);
    mouseLine.setAttributeNS(null, "x2", ""+x);

    // Update the timestamp for the cursor.
    const ts = getTimeAtMouseX(x);
    const mouseTime = $$<SVGTextElement>("#mouse-text", svg)!;
    mouseTime.setAttributeNS(null, "x", ""+x);
    mouseTime.innerHTML = ts.toLocaleTimeString();

    // If we're selecting, update the selection box.
    if (dragStartX !== undefined) {
      let x1 = dragStartX;
      let x2 = x;
      if (x2 < x1) {
        x2 = x1;
        x1 = x;
      }
      const selectRect = $$<SVGRectElement>("#select-rect", svg)!;
      const selectText = $$<SVGTextElement>("#select-text", svg)!;
      const selectLineStart = $$<SVGLineElement>("#select-line-start", svg)!;
      const selectLineEnd = $$<SVGLineElement>("#select-line-end", svg)!;

      selectRect.setAttributeNS(null, "x", ""+x1);
      selectRect.setAttributeNS(null, "width", ""+(x2-x1));
      selectLineStart.setAttributeNS(null, "x1", ""+x1);
      selectLineStart.setAttributeNS(null, "x2", ""+x1);
      selectLineEnd.setAttributeNS(null, "x1", ""+x2);
      selectLineEnd.setAttributeNS(null, "x2", ""+x2);

      // Update the selected time range label.
      const selectedDuration =
          getTimeAtMouseX(x2).getTime() - getTimeAtMouseX(x1).getTime();
      selectText.setAttributeNS(null, "x", ""+(x1+x2)/2);
      selectText.innerHTML = strDuration(selectedDuration / 1000);

      // The cursor time label interferes with the selected time labels.
      // make it disappear if we're actively selecting.
      mouseTime.innerHTML = "";
    }
  };

  // Create a selection box when the mouse is clicked and dragged. This is a
  // helper function used by the mousedown callback function.
  const mouseSelectStart = (e: MouseEvent) => {
    const svg = $$<SVGElement>("svg", container);
    if (!svg) {
      return;
    }
    const selectRect = $$<SVGRectElement>("#select-rect", svg)!;
    const selectLineStart = $$<SVGLineElement>("#select-line-start", svg)!;
    const selectLineEnd = $$<SVGLineElement>("#select-line-end", svg)!;

    const x = getMouseX(e);

    selectRect.setAttributeNS(null, "x", ""+x);
    selectLineStart.setAttributeNS(null, "x1", ""+x);
    selectLineStart.setAttributeNS(null, "x2", ""+x);
    selectLineEnd.setAttributeNS(null, "x1", ""+x);
    selectLineEnd.setAttributeNS(null, "x2", ""+x);

    dragStartX = x;
  };

  const mouseOut = (e: MouseEvent) => {
    dragStartX = undefined;
  }

  // Render.
  render(svg`
    <svg
        width="${totalWidth}"
        height="${totalHeight}"
        @mousemove="${(e:MouseEvent) => {mouseMoved(e)}}"
        @mousedown="${(e:MouseEvent) => {mouseSelectStart(e)}}"
        @mouseup="${(e:MouseEvent) => {mouseOut(e)}}"
        @mouseleave="${(e:MouseEvent) => {mouseOut(e)}}"
        >
      ${normEpochs.map((epoch: Epoch) => svg`
        <rect
            x="${epoch.x}"
            y="${epoch.y}"
            width="${epoch.width}"
            height="${epoch.height}"
            fill="${epoch.color}"
            >
        </rect>
      `)}
      ${labels.map((lbl: Label) => svg`
        <text
            alignment-baseline="middle"
            text-anchor="end"
            style="-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;"
            x="${lbl.x}"
            y="${lbl.y}"
            width="${lbl.width}"
            height="${labelHeight}"
            font-family="${labelFontFamily}"
            font-size="${labelFontSize}"
            >
          ${lbl.text}
        </text>
      `)}
      ${blocks.map((block: DisplayBlock) => svg`
        <rect
            stroke="none"
            x="${block.x}"
            y="${block.y}"
            width="${block.width}"
            height="${block.height}"
            fill="${block.color}"
            >
          <title>${block.title}</title>
        </rect>
      `)}
      ${borders.map((b: Border) => svg`
        <line
            stroke="black"
            stroke-width="hairline"
            x1="${b.x1}"
            y1="${b.y1}"
            x2="${b.x2}"
            y2="${b.y2}"
            >
        </line>
      `)}
      ${rulerTicks.map((tick: RulerTick) => svg`
        <line
            stroke="black"
            stroke-width="hairline"
            x1="${tick.x1}"
            y1="${tick.y1}"
            x2="${tick.x2}"
            y2="${tick.y2}"
            >
        </line>
      `)}
      ${rulerTexts.map((text: RulerText) => svg`
        <text
            alignment-baseline="middle"
            text-anchor="end"
            style="-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;"
            x="${text.x}"
            y="${text.y}"
            font-family="${labelFontFamily}"
            font-size="${labelFontSize}"
            transform="rotate(${rulerLabelRotation} ${text.rotationX} ${text.rotationY})"
            >
          ${text.text}
        </text>
      `)}
      <line
          id="mouse-line"
          stroke="black"
          stroke-width="hairline"
          x1="-1000"
          y1="${blocksStartY - 10}"
          x2="-1000"
          y2="${blocksStartY + blocksHeight}"
          >
      </line>
      <text
          id="mouse-text"
          alignment-baseline="bottom"
          text-anchor="middle"
          style="-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;"
          x="-1000"
          y="${blocksStartY - 20}"
          font-family="${labelFontFamily}"
          font-size="${labelFontSize}"
          >
      </text>
      <rect
          id="select-rect"
          fill="red"
          fill-opacity="0.2"
          x="-1000"
          y="${blocksStartY}"
          width="0"
          height="${blocksHeight}"
          >
      </rect>
      <text
          id="select-text"
          alignment-baseline="bottom"
          text-anchor="middle"
          style="-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;"
          x="-1000"
          y="${blocksStartY - 10}"
          font-family="${labelFontFamily}"
          font-size="${labelFontSize}"
          >
      </text>
      <line
          id="select-line-start"
          stroke="black"
          stroke-width="hairline"
          x1="-1000"
          y1="${blocksStartY - 10}"
          x2="-1000"
          y2="${blocksStartY}"
          >
      </line>
      <line
          id="select-line-end"
          stroke="black"
          stroke-width="hairline"
          x1="-1000"
          y1="${blocksStartY - 10}"
          x2="-1000"
          y2="${blocksStartY}"
          >
      </line>
    </svg>
  `, container);
}
