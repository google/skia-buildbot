// Abstracts out the server component in app.html.
//

var DebugCanvas = {
  // setBreakpoint runs the skp starting at opOffset until the pixel
  // located at (x,y) changes.
  //
  // Returns a promise that resolves with information on the changed pixel:
  //
  //   {
  //     "startColor":[255,79,54,200],
  //     "endColor":[255,144,206,1],
  //     "endOp":340
  //   }
  setBreakpoint: function(opOffset, x, y) {
    return sk.get("./break/" + opOffset + "/" + x + "/" + y).then(JSON.parse);
  },

  // getInfo returns a promise that resolves with information on
  // the view matrix and clip rect of the form:
  //
  //   {
  //     "ViewMatrix":[[1,0,0],[0,1,0],[0,0,1]],
  //     "ClipRect":[0,0,256,256],
  //   }
  getInfo: function(index) {
    if (index === undefined) {
      return sk.get("./info").then(JSON.parse);
    } else  {
      return sk.get("./info/" + index).then(JSON.parse);
    }
  },

  // getCmd returns a promise that resolves to a list of all the commands
  //   and the current state of the canvas:
  //
  //   {
  //     mode: "cpu",
  //     drawGpuOpBounds: false,
  //     colorMode: 0,
  //     version: 1,
  //     commands: [{command: "BeginDrawPicture", visible: true}, ...]
  //   }
  getCmd: function() {
      return sk.get("./cmd").then(JSON.parse);
  },

  // Sets the clip.
  //
  // b is a boolean.
  //
  // Return a promise that resolves when the change is complete.
  setClip: function(b) {
    var alpha = b ? 128 : 0;
    return sk.post("./clipAlpha/" + alpha);
  },

  // setTextBounds sets if text bounds are displayed.
  //
  // b is a boolean.
  //
  // Return a promise that resolves when the change is complete.
  setTextBounds: function(b) {
    var checked = b ? 1 : 0;
    return sk.post("./textBounds/" + checked);
  },

  // toggleOp turns the op at index off or on based on the boolean 'b'.
  //
  // Returns a promise that resolved when the change is complete.
  toggleOp: function(index, b) {
    var value = b ? "1" : "0";
    return sk.post("./cmd/" + index + "/" + value, "");
  },

  // setColorMode sets the color mode where mode is an integer, one of:
  //
  //   0 = Linear 32-bit
  //   1 = sRGB 32-bit
  //   2 = Linear half-precision float
  //
  // Returns a promise that resolved when the change is complete.
  setColorMode: function(mode) {
    return sk.post("./colorMode/" + mode, "");
  },

  // setGPU turns on or off the GPU based on the boolean b.
  //
  // Returns a promise that resolved when the change is complete.
  setGPU: function(b) {
    return sk.post("./enableGPU/" + (b ? 1 : 0), "");
  },

  // setGPUBounds turn on or off the display of the GPU bounds.
  //
  // Returns a promise that resolved when the change is complete.
  setGPUBounds: function (b) {
    return sk.post("./gpuOpBounds/" + (b ? 1 : 0), "");
  },
};
