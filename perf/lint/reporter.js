// A reporter module to be used by JsHint for formatting the output.
module.exports = {
  reporter: function(res) {
    const len = res.length;
    let str = '';

    res.forEach((r) => {
      const file = r.file;
      const err = r.error;

      str += `${file}:${err.line}:${err.character} ${err.reason}\n`;
    });

    if (str) {
      process.stdout.write(`${str}\n${len} error${
        (len === 1) ? '' : 's'}\n`);
    }
  },
};
