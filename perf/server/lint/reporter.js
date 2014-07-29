"use strict";

// A reporter module to be used by JsHint for formatting the output.
module.exports = {
  reporter: function (res) {
    var len = res.length;
    var str = "";

    res.forEach(function (r) {
      var file = r.file;
      var err = r.error;

      str += file + ":" + err.line + ":" + err.character + " " + err.reason + "\n";
    });

    if (str) {
      process.stdout.write(str + "\n" + len + " error" +
        ((len === 1) ? "" : "s") + "\n");
    }
  }
};
