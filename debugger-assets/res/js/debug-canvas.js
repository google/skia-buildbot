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
};
