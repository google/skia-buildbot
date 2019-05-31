/**
 * Tools for displaying Gantt charts.
 */

function gantt(svg) {
  const rv = {
    _svg: svg,
  };

  /**
   * Set the list of tasks on the chart.
   */
  rv.tasks = function(tasks) {
    this._tasks = tasks;
    return this;
  };

  /**
   * Set the list of categories to display. Any tasks with categories not in
   * this list will not be displayed. If this is not called, or if it's called
   * with an empty list, all categories are displayed.
   */
  rv.categories = function(categories) {
    this._categories = categories;
    return this;
  };

  /**
   * Set a list of epochs, given as Date objects, to mark on the chart. These
   * are distinguished by a change in background color.
   */
  rv.epochs = function(epochs) {
    this._epochs = epochs;
    return this;
  };

  /**
   * Draw the chart into the given SVG element.
   */
  rv.draw = function() {
    // Arrange the tasks into rows by category.
    const tasksByCategory = {};
    for (const task of this._tasks) {
      if (!tasksByCategory[task.category]) {
        tasksByCategory[task.category] = [];
      }
      tasksByCategory[task.category].push(task);
    }

    // Finalize the list of categories.
    let categories = this._categories;
    if (!categories) {
      categories = [];
      for (const category in tasksByCategory) {
        categories.push(category);
      }
    }

    // Calculate label offset.
    const boundingRect = this._svg.getBoundingClientRect();
    const totalWidth = boundingRect.width;
    const totalHeight = boundingRect.height;
    const chartMarginLeft = 5;
    const chartMarginRight = 110;
    const chartMarginY = 5;
    const rulerHeight = 78;
    const rulerLabelMarginRight = 5;
    const rulerLabelRotation = -30;
    const rulerTickLength = 15;
    const mouseoverHeight = 50;
    const blocksHeight = totalHeight - rulerHeight - mouseoverHeight - 2*chartMarginY;
    const rowHeight = blocksHeight / categories.length;
    const blockHeight = rowHeight * 0.8;
    const labelFontFamily = 'Arial';
    const labelFontSize = 11;
    const labelHeight = 20;
    const blockMarginY = (rowHeight - blockHeight) / 2;
    const labelMarginY = (rowHeight - labelHeight) / 2;

    // This is a temporary throwaway canvas which is only used for measuring
    // text.
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    ctx.font = labelFontSize + 'px ' + labelFontFamily;
    let labelWidth = 0;
    for (const category of categories) {
      let width = ctx.measureText(category).width;
      if (width > labelWidth) {
        labelWidth = width;
      }
    }
    const labelMarginRight = 10;
    const blockStartX = chartMarginLeft + labelWidth + labelMarginRight;
    const blockStartY = chartMarginY + mouseoverHeight;
    const blocksWidth = totalWidth - blockStartX - chartMarginRight;

    // Find the time range which encompasses all tasks.
    let tStart = Number.MAX_SAFE_INTEGER;
    let tEnd = 0;
    for (const category of categories) {
      const tasks = tasksByCategory[category] || [];
      for (const task of tasks) {
        const start = task.start.getTime();
        if (start < tStart) {
          tStart = start;
        }
        if (start > tEnd) {
          tEnd = start;
        }
        const end = task.end.getTime();
        if (end < tStart) {
          tStart = start;
        }
        if (end > tEnd) {
          tEnd = end;
        }
      }
    }
    if (tStart > tEnd) {
      console.warn('End timestamp is after start!');
      tEnd = tStart + 1000; // Just to give the chart some area.
    }
    const duration = tEnd - tStart;

    // Organize the tasks into rows.
    const blocks = [];
    const labels = [];
    for (let i = 0; i < categories.length; i++) {
      const category = categories[i];
      labels.push({
        text: category,
        x: chartMarginLeft + labelWidth,
        y: blockStartY + i * rowHeight + labelHeight / 2 + labelMarginY,
        width: labelWidth,
        height: labelHeight,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
      });
      for (const task of tasksByCategory[category]) {
        const start = task.start.getTime();
        const end = task.end.getTime();
        blocks.push({
          x: blockStartX + blocksWidth * (start - tStart) / duration,
          y: blockStartY + i * rowHeight + blockMarginY,
          width: blocksWidth * (end - start) / duration,
          height: blockHeight,
          title: sk.human.strDuration((end - start) / 1000),
        });
      }
    }
    this._layoutCategories = labels;
    this._layoutTasks = blocks;

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
    const epochs = this._epochs || [];
    // Ensure that there's an epoch block which reaches to the end of the chart.
    epochs.push(new Date(tEnd));
    const normEpochs = [];
    const epochColors = [
      '#EFEFEF',
      '#EAEAEA',
    ];
    let lastX = blockStartX;
    for (let i = 0; i < epochs.length; i++) {
      const epoch = epochs[i].getTime();
      if (epoch >= tStart && epoch <= tEnd) {
        const x = blockStartX + blocksWidth * (epoch - tStart) / duration;
        normEpochs.push({
          x: lastX,
          y: blockStartY,
          width: x - lastX,
          height: blocksHeight,
          color: epochColors[i % epochColors.length],
        });
        lastX = x;
      }
    }
    this._layoutEpochs = normEpochs;

    const rulerTicks = [];
    const rulerTexts = [];
    let lastDate = new Date(ticks[0]).getDate();
    for (const tick of ticks) {
      const x = blockStartX + blocksWidth * (tick - tStart) / duration;
      const y1 = blockStartY + blocksHeight;
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
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        rotationDegrees: rulerLabelRotation,
        rotationX: x,
        rotationY: y2,
        text: d.getDate() === lastDate ? d.toLocaleTimeString() : d.toLocaleString(),
      });
      lastDate = d.getDate();
    }
    this._layoutRulerTexts = rulerTexts;
    this._layoutRulerTicks = rulerTicks;

    // Draw border lines around the chart.
    this._layoutBorders = [
      {
        x1: blockStartX,
        y1: blockStartY,
        x2: blockStartX,
        y2: blockStartY + blocksHeight,
      },
      {
        x1: blockStartX,
        y1: blockStartY + blocksHeight,
        x2: blockStartX + blocksWidth,
        y2: blockStartY + blocksHeight,
      },
    ];

    // Helper function for finding the x-value and timestamp given a mouse
    // event.
    this._layoutGetMouseX = function(e) {
      // Convert event x-coordinate to a coordinate within the chart area.
      let x = e.clientX - boundingRect.x;
      if (x < blockStartX) {
        x = blockStartX;
      } else if (x > totalWidth - chartMarginRight) {
        x = totalWidth - chartMarginRight;
      }
      // Find the nearest block border; if we're close enough, snap the line.
      let nearest = 0;
      let nearestDist = blocksWidth;
      for (const block of this._layoutTasks) {
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
      const snapThreshold = 15;
      if (nearestDist < snapThreshold) {
        x = nearest;
      }
      return x;
    };

    // Helper function for finding the timestamp associated with the given
    // x-coordinate on the chart.
    this._layoutGetSelectedTime = function(x) {
      return new Date(tStart + ((x - blockStartX) / blocksWidth) * duration);
    };

    // Helper function for creating a human-readable timestamp string. The
    // built-in toLocaleString() and toLocaleTimeString() functions do not
    // include milliseconds, which we want to see here.
    this._layoutFormatTime = function(ts) {
      return ts.toLocaleDateString() + ' ' +
          ts.getHours().toString().padStart(2, "0") + ':' +
          ts.getMinutes().toString().padStart(2, "0") + ':' +
          ts.getSeconds().toString().padStart(2, "0") + '.' +
          ts.getMilliseconds();
    };

    // Create a vertical line used on mouseover. This is a helper function used
    // by the mousemove callback function.
    this._layoutUpdateMouse = function(e) {
      const x = this._layoutGetMouseX(e);
      const mouseLine = {
        x: x,
        y1: blockStartY - 10,
        y2: blockStartY + blocksHeight,
      };
      const ts = this._layoutGetSelectedTime(x);
      const mouseTime = {
        x: x,
        y: mouseLine.y1 - 10,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        text: this._layoutFormatTime(ts),
      };

      this._layoutMouseLine = [mouseLine];
      this._layoutMouseTime = [mouseTime];

      // Update the selection box, if it's active.
      // this._layoutSelectBoxOrigin could be zero; compare against undefined.
      if (this._layoutSelectBoxOrigin !== undefined) {
        let x1 = this._layoutSelectBoxOrigin;
        let x2 = x;
        if (x2 < x1) {
          x2 = x1;
          x1 = x;
        }
        this._layoutSelectBox[0].x = x1;
        this._layoutSelectBox[0].width = x2 - x1;

        // Update the selected time range label.
        const selectedDuration = this._layoutGetSelectedTime(x2) - this._layoutGetSelectedTime(x1);
        this._layoutSelectedTimeRange[0].x1 = x1;
        this._layoutSelectedTimeRange[0].x2 = x2;
        this._layoutSelectedTimeRange[0].text = sk.human.strDuration(selectedDuration / 1000);

        // The cursor time label interferes with the selected time labels.
        // make it disappear if we're actively selecting.
        this._layoutMouseTime = [];
      }

      this.layout();
    };

    // Create a selection box when the mouse is clicked and dragged. This is a
    // helper function used by the mousedown callback function.
    this._layoutStartSelection = function(e) {
      const x = this._layoutGetMouseX(e);
      this._layoutSelectBox = [{
        x: x,
        y: blockStartY,
        width: 0,
        height: blocksHeight,
      }];
      this._layoutSelectBoxOrigin = x;
      const ts = this._layoutGetSelectedTime(x);
      this._layoutSelectedTimeRange = [{
        x1: x,
        x2: x,
        y1: blockStartY - 10,
        y2: blockStartY,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        text: '',
      }];
      this.layout();
    };

    // Set the mouse line location.
    if (this._layoutMouseLine && this._layoutMouseLine.length > 0) {
      this._layoutMouseLine[0].y2 = blockStartY + blocksHeight;
    } else {
      this._layoutMouseLine = [];
    }
    this._layoutMouseTime = this._layoutMouseTime || [];

    // Set the layout selection box location.
    if (this._layoutSelectBox && this._layoutSelectBox.length > 0) {
      this._layoutSelectBox[0].height = blocksHeight;
    } else {
      this._layoutSelectBox = [];
    }
    if (this._layoutSelectedTimeRange && this._layoutSelectedTimeRange.length > 0) {
      this._layoutSelectedTimeRange[0].y1 = blockStartY - 10;
      this._layoutSelectedTimeRange[0].y2 = blockStartY;
    } else {
      this._layoutSelectedTimeRange = [];
    }

    this.layout();
  };

  /**
   * Perform the layout.
   */
  rv.layout = function() {
    // Draw using d3.
    const d3svg = d3.select(this._svg);

    // Draw background blocks for each epoch.
    const epochRects = d3svg.selectAll('rect.epoch').data(this._layoutEpochs);
    epochRects.enter().append('svg:rect')
        .attr('class', 'epoch');
    epochRects
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('width', function(d) { return d.width; })
        .attr('height', function(d) { return d.height; })
        .attr('fill', function(d) { return d.color; });
    epochRects.exit().remove();

    // Draw task labels.
    const labelTexts = d3svg.selectAll('text.label').data(this._layoutCategories);
    labelTexts.enter().append('svg:text')
        .attr('class', 'label')
        .attr('alignment-baseline', 'middle')
        .attr('text-anchor', 'end')
        .attr('style', '-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;');
    labelTexts
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('width', function(d) { return d.width; })
        .attr('height', function(d) { return d.height; })
        .attr('font-family', function(d) { return d.fontFamily; })
        .attr('font-size', function(d) { return d.fontSize; })
        .text(function(d) { return d.text; });
    labelTexts.exit().remove();

    // Draw task rects.
    const taskRects = d3svg.selectAll('rect.task').data(this._layoutTasks);
    taskRects.enter().append('svg:rect')
        .attr('class', 'task')
        .append('svg:title')
          .attr('class', 'task');
    taskRects
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('width', function(d) { return d.width; })
        .attr('height', function(d) { return d.height; });
    taskRects.exit().remove();
    const taskTexts = d3svg.selectAll('title.task').data(this._layoutTasks);
    taskTexts.text(function(d) { return d.title; });
    taskTexts.exit().remove();

    // Draw borders around the chart area.
    const borderLines = d3svg.selectAll('line.border').data(this._layoutBorders);
    borderLines.enter().append('line')
        .attr('class', 'border')
        .attr('stroke', 'black')
        .attr('stroke-width', 'hairline');
    borderLines
        .attr('x1', function(d) { return d.x1; })
        .attr('y1', function(d) { return d.y1; })
        .attr('x2', function(d) { return d.x2; })
        .attr('y2', function(d) { return d.y2; });
    borderLines.exit().remove();

    // Draw ruler.
    const rulerTickLines = d3svg.selectAll('line.rulerTick').data(this._layoutRulerTicks);
    rulerTickLines.enter().append('line')
        .attr('class', 'rulerTick')
        .attr('stroke', 'black')
        .attr('stroke-width', 'hairline');
    rulerTickLines
        .attr('x1', function(d) { return d.x1; })
        .attr('y1', function(d) { return d.y1; })
        .attr('x2', function(d) { return d.x2; })
        .attr('y2', function(d) { return d.y2; });
    rulerTickLines.exit().remove();
    const rulerTextsSvg = d3svg.selectAll('text.ruler').data(this._layoutRulerTexts);
    rulerTextsSvg.enter().append('svg:text')
        .attr('class', 'ruler')
        .attr('alignment-baseline', 'middle')
        .attr('text-anchor', 'end')
        .attr('style', '-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;');
    rulerTextsSvg
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('width', function(d) { return d.width; })
        .attr('height', function(d) { return d.height; })
        .attr('font-family', function(d) { return d.fontFamily; })
        .attr('font-size', function(d) { return d.fontSize; })
        .attr('transform', function(d) {
            return 'rotate(' + d.rotationDegrees + ' ' + d.rotationX + ' ' + d.rotationY + ')';
        })
        .text(function(d) { return d.text; });
    rulerTextsSvg.exit().remove();

    // Mouse cursor bar.
    const mouseLine = d3svg.selectAll('line.mouse').data(this._layoutMouseLine);
    mouseLine.enter().append('line')
        .attr('class', 'mouse')
        .attr('stroke', 'black')
        .attr('stroke-width', 'hairline');
    mouseLine
        .attr('x1', function(d) { return d.x; })
        .attr('y1', function(d) { return d.y1; })
        .attr('x2', function(d) { return d.x; })
        .attr('y2', function(d) { return d.y2; });
    mouseLine.exit().remove();

    // Mouse cursor time tooltip.
    const mouseoverTime = d3svg.selectAll('text.mouse').data(this._layoutMouseTime);
    mouseoverTime.enter().append('text')
        .attr('class', 'mouse')
        .attr('alignment-baseline', 'bottom')
        .attr('text-anchor', 'middle')
        .attr('style', '-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;');
    mouseoverTime
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('font-family', function(d) { return d.fontFamily; })
        .attr('font-size', function(d) { return d.fontSize; })
        .text(function(d) { return d.text; });
    mouseoverTime.exit().remove();

    // Selection box.
    const selectBox = d3svg.selectAll('rect.selectBox').data(this._layoutSelectBox);
    selectBox.enter().append('rect')
        .attr('class', 'selectBox')
        .attr('fill', 'red')
        .attr('fill-opacity', '0.2');
    selectBox
        .attr('x', function(d) { return d.x; })
        .attr('y', function(d) { return d.y; })
        .attr('width', function(d) { return d.width; })
        .attr('height', function(d) { return d.height; });
    selectBox.exit().remove();

    // Selected times.
    const selectedTimeRangeText = d3svg.selectAll('text.selectedTimeRange').data(this._layoutSelectedTimeRange);
    selectedTimeRangeText.enter().append('text')
        .attr('class', 'selectedTimeRange')
        .attr('alignment-baseline', 'bottom')
        .attr('text-anchor', 'middle')
        .attr('style', '-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;');
    selectedTimeRangeText
        .attr('x', function(d) { return (d.x2 + d.x1) / 2; })
        .attr('y', function(d) { return d.y1; })
        .attr('font-family', function(d) { return d.fontFamily; })
        .attr('font-size', function(d) { return d.fontSize; })
        .text(function(d) { return d.text; });
    selectedTimeRangeText.exit().remove();
    const selectedTimeRangeLine1 = d3svg.selectAll('line.selectedTimeRange1').data(this._layoutSelectedTimeRange);
    selectedTimeRangeLine1.enter().append('line')
        .attr('class', 'selectedTimeRange1')
        .attr('stroke', 'black')
        .attr('stroke-width', 'hairline');
    selectedTimeRangeLine1
        .attr('x1', function(d) { return d.x1; })
        .attr('y1', function(d) { return d.y1; })
        .attr('x2', function(d) { return d.x1; })
        .attr('y2', function(d) { return d.y2; });
    selectedTimeRangeLine1.exit().remove();
    const selectedTimeRangeLine2 = d3svg.selectAll('line.selectedTimeRange2').data(this._layoutSelectedTimeRange);
    selectedTimeRangeLine2.enter().append('line')
        .attr('class', 'selectedTimeRange2')
        .attr('stroke', 'black')
        .attr('stroke-width', 'hairline');
    selectedTimeRangeLine2
        .attr('x1', function(d) { return d.x2; })
        .attr('y1', function(d) { return d.y1; })
        .attr('x2', function(d) { return d.x2; })
        .attr('y2', function(d) { return d.y2; });
    selectedTimeRangeLine2.exit().remove();
  };

  /**
   * Handler for mousemove events.
   */
  rv._mouseMove = function(e) {
    if (this._layoutUpdateMouse) {
      this._layoutUpdateMouse(e);
    }
  };
  rv._svg.addEventListener('mousemove', rv._mouseMove.bind(rv));

  /**
   * Handler for mousedown events.
   */
  rv._mouseDown = function(e) {
    if (this._layoutStartSelection) {
      this._layoutStartSelection(e);
    }
  };
  rv._svg.addEventListener('mousedown', rv._mouseDown.bind(rv));

  /**
   * Handler for mouseup and mouseleave events.
   */
  rv._mouseUp = function(e) {
    this._layoutSelectBoxOrigin = undefined;
  };
  rv._svg.addEventListener('mouseup', rv._mouseUp.bind(rv));
  rv._svg.addEventListener('mouseleave', rv._mouseUp.bind(rv));
  return rv;
}
