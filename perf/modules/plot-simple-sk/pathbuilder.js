/** @class Builds the Path2D objects that describe the trace and the dots for a given
*   set of scales.
*/
class PathBuilder {
  constructor(xRange, yRange, radius) {
    this.xRange = xRange;
    this.yRange = yRange;
    this.radius = radius;
    this.linePath = new Path2D();
    this.dotsPath = new Path2D();
  }

  /**
       * Add a point to plot to the path.
       *
       * @param {Number} x - X coordinate in source coordinates.
       * @param {Number} y - Y coordinate in source coordinates.
       */
  add(x, y) {
    // Convert source coord into canvas coords.
    const cx = this.xRange(x);
    const cy = this.yRange(y);

    if (x === 0) {
      this.linePath.moveTo(cx, cy);
    } else {
      this.linePath.lineTo(cx, cy);
    }
    this.dotsPath.moveTo(cx + this.radius, cy);
    this.dotsPath.arc(cx, cy, this.radius, 0, 2 * Math.PI);
  }

  /**
       * Returns the Arrays of Path2D objects that represent all the traces.
       *
       * @returns {Object}
       */
  paths() {
    return {
      _linePath: this.linePath,
      _dotsPath: this.dotsPath,
    };
  }
}

export { PathBuilder as default };
