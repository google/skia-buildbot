import { kdTree } from './kd';

/**
 * @class Builds a kdTree for searcing for nearest points to the mouse.
 */
class SearchBuilder {
  constructor() {
    this.points = [];
  }

  /**
     * Add a point to the kdTree.
     *
     * @param {Number} x - X coordinate in source coordinates.
     * @param {Number} y - Y coordinate in source coordinates.
     * @param {String} name - The trace name.
     */
  add(x, y, name) {
    if (name.startsWith(SPECIAL)) {
      return;
    }
    this.points.push(
      {
        x,
        y,
        name,
      },
    );
  }

  /**
     * Returns a kdTree that contains all the points being plotted.
     *
     * @returns {kdTree}
     */
  kdTree() {
    const distance = (a, b) => {
      const dx = (a.x - b.x);
      const dy = (a.y - b.y);
      return dx * dx + dy * dy;
    };

    return new kdTree(this.points, distance, ['x', 'y']);
  }
}

export { SearchBuilder as default };
