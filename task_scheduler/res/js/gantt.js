/**
 * Tools for displaying Gantt charts.
 */

function gantt(svg) {
  var rv = {
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
    var tasksByCategory = {};
    for (var i = 0; i < this._tasks.length; i++) {
      var task = this._tasks[i];
      if (!tasksByCategory[task.category]) {
        tasksByCategory[task.category] = [];
      }
      tasksByCategory[task.category].push(task);
    }

    // Finalize the list of categories and their order.
    var categories = this._categories;
    if (!categories) {
      categories = [];
      for (var category in tasksByCategory) {
        categories.push(category);
      }
    }

    // Calculate label offset.
    var totalWidth = this._svg.getBoundingClientRect().width;
    var totalHeight = this._svg.getBoundingClientRect().height;
    var chartMarginLeft = 5;
    var chartMarginRight = 110;
    var chartMarginY = 5;
    var rulerHeight = 78;
    var rulerLabelMarginRight = 5;
    var rulerLabelRotation = 30;
    var rulerTickLength = 15;
    var mouseoverHeight = 50;
    var blocksHeight = totalHeight - rulerHeight - mouseoverHeight - 2*chartMarginY;
    var rowHeight = blocksHeight / categories.length;
    var blockHeight = rowHeight * 0.8;
    var labelFontFamily = "Arial";
    var labelFontSize = 11;
    var labelHeight = 20;
    var blockMarginY = (rowHeight - blockHeight) / 2;
    var labelMarginY = (rowHeight - labelHeight) / 2;
    var canvas = document.createElement("canvas");
    var ctx = canvas.getContext("2d");
    ctx.font = labelFontSize + "px " + labelFontFamily;
    var labelWidth = 0;
    for (var i = 0; i < categories.length; i++) {
      var category = categories[i];
      var width = ctx.measureText(category).width;
      if (width > labelWidth) {
        labelWidth = width;
      }
    }
    var labelMarginRight = 10;
    var blockStartX = chartMarginLeft + labelWidth + labelMarginRight;
    var blockStartY = chartMarginY + mouseoverHeight;
    var blocksWidth = totalWidth - blockStartX - chartMarginRight;

    // Timestamp normalization.
    var tStart = Date.now();
    var tEnd = 0;
    for (var i = 0; i < categories.length; i++) {
      var category = categories[i];
      var tasks = tasksByCategory[category] || [];
      for (var j = 0; j < tasks.length; j++) {
        var task = tasks[j];
        var start = task.start.getTime();
        if (start < tStart) {
          tStart = start;
        }
        if (start > tEnd) {
          tEnd = start;
        }
        var end = task.end.getTime();
        if (end < tStart) {
          tStart = start;
        }
        if (end > tEnd) {
          tEnd = end;
        }
      }
    }
    if (tStart > tEnd) {
      console.log("warning: end is after start");
      tEnd = tStart + 1000; // Just to give the chart some area.
    }
    
    var duration = tEnd - tStart;

    // Organize the tasks into rows.
    var blocks = [];
    var labels = [];
    for (var i = 0; i < categories.length; i++) {
      var category = categories[i];
      labels.push({
        text: category,
        x: chartMarginLeft + labelWidth,
        y: blockStartY + i * rowHeight + labelHeight / 2 + labelMarginY,
        width: labelWidth,
        height: labelHeight,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
      });
      var tasks = tasksByCategory[category];
      for (var j = 0; j < tasks.length; j++) {
        var task = tasks[j];
        var start = task.start.getTime();
        var end = task.end.getTime();
        blocks.push({
          start: task.start,
          end: task.end,
          x: blockStartX + blocksWidth * (start - tStart) / duration,
          y: blockStartY + i * rowHeight + blockMarginY,
          width: blocksWidth * (end - start) / duration,
          height: blockHeight,
        });
      }
    }
    this._layoutCategories = labels;
    this._layoutTasks = blocks;

    // Create the ruler.
    // We want approximately one tick every 50-100 px.
    var numTargetTicks = blocksWidth / 75;
    var approxTickSize = duration / numTargetTicks;
    // Round the tick size to the nearest multiple of an appropriate unit.
    // Timestamps are in milliseconds.
    var units = [
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
    var lowestDist = -1;
    var actualTickSize = units[0];
    for (var i = 0; i < units.length; i++) {
      var unit = units[i];
      var dist = Math.abs(approxTickSize - unit);
      if (lowestDist == -1 || dist < lowestDist) {
        lowestDist = dist;
        actualTickSize = unit;
      }
    }
    // Find an "anchor" for the ticks to start.
    var tickAnchor = new Date(tStart);
    tickAnchor.setHours(0);
    tickAnchor.setMinutes(0);
    tickAnchor.setSeconds(0);
    tickAnchor.setMilliseconds(0);
    // Create the ticks.
    var tick = tickAnchor.getTime() + Math.ceil((tStart - tickAnchor.getTime()) / actualTickSize) * actualTickSize;
    var ticks = [];
    while (tick < tEnd) {
      ticks.push(tick);
      tick += actualTickSize;
    }

    // Create background blocks for each epoch.
    var epochs = this._epochs || [];
    // Ensure that there's an epoch block which reaches to the end of the chart.
    epochs.push(new Date(tEnd));
    var normEpochs = [];
    var lastX = blockStartX;
    var epochColors = [
      "#EFEFEF",
      "#EAEAEA",
    ];
    for (var i = 0; i < epochs.length; i++) {
      var epoch = epochs[i].getTime();
      if (epoch >= tStart && epoch <= tEnd) {
        var x = blockStartX + blocksWidth * (epoch - tStart) / duration;
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

    var rulerTicks = [];
    var rulerTexts = [];
    var lastDate = new Date(ticks[0]).getDate();
    for (var i = 0; i < ticks.length; i++) {
      var tick = ticks[i];
      var x = blockStartX + blocksWidth * (tick - tStart) / duration;
      var y1 = blockStartY + blocksHeight;
      var y2 = y1 + rulerTickLength;
      rulerTicks.push({
        x1: x,
        y1: y1,
        x2: x,
        y2: y2,
      });
      var d = new Date(tick);
      rulerTexts.push({
        x: x - rulerLabelMarginRight,
        y: y2,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        rotationDegrees: rulerLabelRotation,
        rotationX: x,
        rotationY: y2,
        text: d.getDate() == lastDate ? d.toLocaleTimeString() : d.toLocaleString(),
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
      // Convert event x-coordinate to a coordinate within the chart area,
      // and derive a timestamp from it.
      var x = e.clientX - 10; // TODO: Why is this shift necessary?
      if (x < blockStartX) {
        x = blockStartX;
      } else if (x > totalWidth - chartMarginRight) {
        x = totalWidth - chartMarginRight;
      }
      // Find the nearest block border; if we're close enough, snap the line.
      var nearest = 0;
      var nearestDist = blocksWidth;
      for (var i = 0; i < this._layoutTasks.length; i++) {
        var block = this._layoutTasks[i];
        var dist = Math.abs(block.x - x);
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
      var snapThreshold = 15;
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

    // Helper function for creating a human-readable timestamp string.
    this._layoutFormatTime = function(ts) {
      return (ts.getMonth() + 1) + "/" + ts.getDate() + "/" + ts.getFullYear() + " " + ts.getHours() + ":" + ts.getMinutes() + ":" + ts.getSeconds() + "." + ts.getMilliseconds();
    };

    // Create a vertical line used on mouseover. This is a helper function used
    // by the mousemove callback function.
    this._layoutUpdateMouse = function(e) {
      var x = this._layoutGetMouseX(e);
      var mouseLine = {
        x: x,
        y1: blockStartY - 10,
        y2: blockStartY + blocksHeight,
      };
      var ts = this._layoutGetSelectedTime(x);
      var mouseTime = {
        x: x,
        y: mouseLine.y1 - 10,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        text: this._layoutFormatTime(ts),
      };

      this._layoutMouseLine = [mouseLine];
      this._layoutMouseTime = [mouseTime];

      // Update the selection box, if it's active.
      if (this._layoutSelectBoxOrigin !== undefined) {
        var x1 = this._layoutSelectBoxOrigin;
        var x2 = x;
        if (x2 < x1) {
          x2 = x1;
          x1 = x;
        }
        this._layoutSelectBox[0].x = x1;
        this._layoutSelectBox[0].width = x2 - x1;

        // Update the selected time range label.
        var selectedDuration = this._layoutGetSelectedTime(x2) - this._layoutGetSelectedTime(x1);
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
      var x = this._layoutGetMouseX(e);
      this._layoutSelectBox = [{
        x: x,
        y: blockStartY,
        width: 0,
        height: blocksHeight,
      }];
      this._layoutSelectBoxOrigin = x;
      var ts = this._layoutGetSelectedTime(x);
      this._layoutSelectedTimeRange = [{
        x1: x,
        x2: x,
        y1: blockStartY - 10,
        y2: blockStartY,
        fontFamily: labelFontFamily,
        fontSize: labelFontSize,
        text: "",
      }];
      this.layout();
    };

    // Set the mouse line location.
    if (this._layoutMouseLine) {
      this._layoutMouseLine[0].y2 = blockStartY + blocksHeight;
    } else {
      this._layoutMouseLine = [];
    }
    this._layoutMouseTime = this._layoutMouseTime || [];

    // Set the layout selection box location.
    if (this._layoutSelectBox) {
      this._layoutSelectBox[0].height = blocksHeight;
    } else {
      this._layoutSelectBox = [];
    }
    if (this._layoutSelectedTimeRange) {
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
    var d3svg = d3.select(this._svg);

    // Draw background blocks for each epoch.
    var epochRects = d3svg.selectAll("rect.epoch").data(this._layoutEpochs);
    epochRects.enter().append("svg:rect")
        .attr("class", "epoch");
    epochRects
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("width", function(data) { return data.width; })
        .attr("height", function(data) { return data.height; })
        .attr("fill", function(data) { return data.color; });
    epochRects.exit().remove();

    // Draw task labels.
    var labelTexts = d3svg.selectAll("text.label").data(this._layoutCategories);
    labelTexts.enter().append("svg:text")
        .attr("class", "label")
        .attr("alignment-baseline", "middle")
        .attr("text-anchor", "end")
        .attr("style", "-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;");
    labelTexts
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("width", function(data) { return data.width; })
        .attr("height", function(data) { return data.height; })
        .attr("font-family", function(data) { return data.fontFamily; })
        .attr("font-size", function(data) { return data.fontSize; })
        .text(function(data) { return data.text; });
    labelTexts.exit().remove();

    // Draw task blocks.
    var blockRects = d3svg.selectAll("rect.block").data(this._layoutTasks);
    blockRects.enter().append("svg:rect")
        .attr("class", "block");
    blockRects
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("width", function(data) { return data.width; })
        .attr("height", function(data) { return data.height; });
    blockRects.exit().remove();

    // Draw borders around the chart area.
    var borderLines = d3svg.selectAll("line.border").data(this._layoutBorders);
    borderLines.enter().append("line")
        .attr("class", "border")
        .attr("stroke", "black")
        .attr("stroke-width", "hairline");
    borderLines
        .attr("x1", function(data) { return data.x1; })
        .attr("y1", function(data) { return data.y1; })
        .attr("x2", function(data) { return data.x2; })
        .attr("y2", function(data) { return data.y2; });
    borderLines.exit().remove();

    // Draw ruler.
    var rulerTickLines = d3svg.selectAll("line.rulerTick").data(this._layoutRulerTicks);
    rulerTickLines.enter().append("line")
        .attr("class", "rulerTick")
        .attr("stroke", "black")
        .attr("stroke-width", "hairline");
    rulerTickLines
        .attr("x1", function(data) { return data.x1; })
        .attr("y1", function(data) { return data.y1; })
        .attr("x2", function(data) { return data.x2; })
        .attr("y2", function(data) { return data.y2; });
    rulerTickLines.exit().remove();
    var rulerTextsSvg = d3svg.selectAll("text.ruler").data(this._layoutRulerTexts);
    rulerTextsSvg.enter().append("svg:text")
        .attr("class", "ruler")
        .attr("alignment-baseline", "middle")
        .attr("text-anchor", "end")
        .attr("style", "-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;");
    rulerTextsSvg
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("width", function(data) { return data.width; })
        .attr("height", function(data) { return data.height; })
        .attr("font-family", function(data) { return data.fontFamily; })
        .attr("font-size", function(data) { return data.fontSize; })
        .attr("transform", function(data) { return "rotate(-" + data.rotationDegrees + " " + data.rotationX + " " + data.rotationY + ")"; })
        .text(function(data) { return data.text; });
    rulerTextsSvg.exit().remove();

    // Mouse cursor bar.
    var mouseLine = d3svg.selectAll("line.mouse").data(this._layoutMouseLine);
    mouseLine.enter().append("line")
        .attr("class", "mouse")
        .attr("stroke", "black")
        .attr("stroke-width", "hairline");
    mouseLine
        .attr("x1", function(data) { return data.x; })
        .attr("y1", function(data) { return data.y1; })
        .attr("x2", function(data) { return data.x; })
        .attr("y2", function(data) { return data.y2; });
    mouseLine.exit().remove();

    // Mouse cursor time tooltip.
    var mouseoverTime = d3svg.selectAll("text.mouse").data(this._layoutMouseTime);
    mouseoverTime.enter().append("text")
        .attr("class", "mouse")
        .attr("alignment-baseline", "bottom")
        .attr("text-anchor", "middle")
        .attr("style", "-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;");
    mouseoverTime
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("font-family", function(data) { return data.fontFamily; })
        .attr("font-size", function(data) { return data.fontSize; })
        .text(function(data) { return data.text; });
    mouseoverTime.exit().remove();

    // Selection box.
    var selectBox = d3svg.selectAll("rect.selectBox").data(this._layoutSelectBox);
    selectBox.enter().append("rect")
        .attr("class", "selectBox")
        .attr("fill", "red")
        .attr("fill-opacity", "0.2");
    selectBox
        .attr("x", function(data) { return data.x; })
        .attr("y", function(data) { return data.y; })
        .attr("width", function(data) { return data.width; })
        .attr("height", function(data) { return data.height; });
    selectBox.exit().remove();

    // Selected times.
    var selectedTimeRangeText = d3svg.selectAll("text.selectedTimeRange").data(this._layoutSelectedTimeRange);
    selectedTimeRangeText.enter().append("text")
        .attr("class", "selectedTimeRange")
        .attr("alignment-baseline", "bottom")
        .attr("text-anchor", "middle")
        .attr("style", "-webkit-user-select:none; -moz-user-select:none; -ms-user-select:none; user-select:none;");
    selectedTimeRangeText
        .attr("x", function(data) { return (data.x2 + data.x1) / 2; })
        .attr("y", function(data) { return data.y1; })
        .attr("font-family", function(data) { return data.fontFamily; })
        .attr("font-size", function(data) { return data.fontSize; })
        .text(function(data) { return data.text; });
    selectedTimeRangeText.exit().remove();
    var selectedTimeRangeLine1 = d3svg.selectAll("line.selectedTimeRange1").data(this._layoutSelectedTimeRange);
    selectedTimeRangeLine1.enter().append("line")
        .attr("class", "selectedTimeRange1")
        .attr("stroke", "black")
        .attr("stroke-width", "hairline");
    selectedTimeRangeLine1
        .attr("x1", function(data) { return data.x1; })
        .attr("y1", function(data) { return data.y1; })
        .attr("x2", function(data) { return data.x1; })
        .attr("y2", function(data) { return data.y2; });
    selectedTimeRangeLine1.exit().remove();
    var selectedTimeRangeLine2 = d3svg.selectAll("line.selectedTimeRange2").data(this._layoutSelectedTimeRange);
    selectedTimeRangeLine2.enter().append("line")
        .attr("class", "selectedTimeRange2")
        .attr("stroke", "black")
        .attr("stroke-width", "hairline");
    selectedTimeRangeLine2
        .attr("x1", function(data) { return data.x2; })
        .attr("y1", function(data) { return data.y1; })
        .attr("x2", function(data) { return data.x2; })
        .attr("y2", function(data) { return data.y2; });
    selectedTimeRangeLine2.exit().remove();
  };

  /**
   * Handler for mousemove events.
   */
  rv._mouseMove = function(e) {
    this._layoutUpdateMouse(e);
  };
  rv._svg.addEventListener("mousemove", rv._mouseMove.bind(rv));

  /**
   * Handler for mousedown events.
   */
  rv._mouseDown = function(e) {
    this._layoutStartSelection(e);
  };
  rv._svg.addEventListener("mousedown", rv._mouseDown.bind(rv));

  /**
   * Handler for mouseup and mouseleave events.
   */
  rv._mouseUp = function(e) {
    this._layoutSelectBoxOrigin = undefined;
  };
  rv._svg.addEventListener("mouseup", rv._mouseUp.bind(rv));
  rv._svg.addEventListener("mouseleave", rv._mouseUp.bind(rv));
  return rv;
}
