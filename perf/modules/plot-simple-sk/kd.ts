/** @module kd
 * A k-d tree implementation, which is used to find the closest point in
 * something like a 2D scatter plot. See https://en.wikipedia.org/wiki/K-d_tree
 * for more details.
 *
 * Forked from https://github.com/Pandinosaurus/kd-tree-javascript and
 * then massively trimmed down to just find the single closest point, and also
 * ported to ES6 syntax, then ported to TypeScript.
 *
 * https://github.com/Pandinosaurus/kd-tree-javascript is a fork of
 * https://github.com/ubilabs/kd-tree-javascript
 *
 * @author Mircea Pricop <pricop@ubilabs.net>, 2012
 * @author Martin Kleppe <kleppe@ubilabs.net>, 2012
 * @author Ubilabs http://ubilabs.net, 2012
 * @license MIT License <http://www.opensource.org/licenses/mit-license.php>
 */

export interface KDPoint {
  x: number;
  y: number;
}

/** @class A single node in the k-d Tree. */
class Node<Point extends KDPoint> {
  obj: Point;
  left: Node<Point> | null;
  right: Node<Point> | null;
  parent: Node<Point> | null;
  dimension: number;

  constructor(obj: Point, dimension: number, parent: Node<Point> | null) {
    this.obj = obj;
    this.left = null;
    this.right = null;
    this.parent = parent;
    this.dimension = dimension;
  }
}

type Keys = keyof KDPoint;

/**
 * @class The k-d tree.
 */
export class KDTree<Point extends KDPoint> {
  private dimensions: Keys[];
  private root: Node<Point> | null;
  private metric: (a: KDPoint, b: KDPoint) => number;
  /**
   * The constructor.
   *
   * @param {Array} points - An array of points, something with the shape
   *     {x:x, y:y}.
   * @param {function} metric - A function that calculates the distance
   *     between two points.
   * @param {Array} dimensions - The dimensions to use in our points, for
   *     example ['x', 'y'].
   */
  constructor(
    points: Point[],
    metric: (a: KDPoint, b: KDPoint) => number,
    dimensions: Keys[]
  ) {
    this.dimensions = dimensions;
    this.metric = metric;
    this.root = this._buildTree(points, 0, null);
  }

  /**
   * Builds the from parent Node on down.
   *
   * @param {Array} points - An array of {x:x, y:y}.
   * @param {Number} depth - The current depth from the root node.
   * @param {Node} parent - The parent Node.
   */
  _buildTree(
    points: Point[],
    depth: number,
    parent: Node<Point> | null
  ): Node<Point> | null {
    // Every step deeper into the tree we switch to using another axis.
    const dim = depth % this.dimensions.length;

    if (points.length === 0) {
      return null;
    }
    if (points.length === 1) {
      return new Node(points[0], dim, parent);
    }

    points.sort((a, b) => a[this.dimensions[dim]] - b[this.dimensions[dim]]);

    const median = Math.floor(points.length / 2);
    const node = new Node(points[median], dim, parent);
    node.left = this._buildTree(points.slice(0, median), depth + 1, node);
    node.right = this._buildTree(points.slice(median + 1), depth + 1, node);

    return node;
  }

  /**
   * Find the nearest Node to the given point.
   *
   * @param {Object} point - {x:x, y:y}
   * @returns {Object} The closest point object passed into the constructor.
   *     We pass back the original object since it might have extra info
   *     beyond just the coordinates, such as trace id.
   */
  nearest(point: KDPoint): Point {
    let bestNode = {
      node: this.root,
      distance: Number.MAX_VALUE,
    };

    const saveNode = (node: Node<Point>, distance: number) => {
      bestNode = {
        node: node,
        distance: distance,
      };
    };

    const nearestSearch = (node: Node<Point>) => {
      const dimension = this.dimensions[node.dimension];
      const ownDistance = this.metric(point, node.obj);

      if (node.right === null && node.left === null) {
        if (ownDistance < bestNode.distance) {
          saveNode(node, ownDistance);
        }
        return;
      }

      let bestChild = null;
      let otherChild = null;
      // If we get here we know that at least one of .left and .right is
      // non-null, so bestChild is guaranteed to be non-null.
      if (node.right === null) {
        bestChild = node.left;
      } else if (node.left === null) {
        bestChild = node.right;
      } else if (point[dimension] < node.obj[dimension]) {
        bestChild = node.left;
        otherChild = node.right;
      } else {
        bestChild = node.right;
        otherChild = node.left;
      }

      nearestSearch(bestChild!);

      if (ownDistance < bestNode.distance) {
        saveNode(node, ownDistance);
      }

      // Find distance to hyperplane.
      const pointOnHyperplane = {
        x: 0,
        y: 0,
      };
      for (let i = 0; i < this.dimensions.length; i++) {
        if (i === node.dimension) {
          pointOnHyperplane[this.dimensions[i]] = point[this.dimensions[i]];
        } else {
          pointOnHyperplane[this.dimensions[i]] = node.obj[this.dimensions[i]];
        }
      }

      // If the hyperplane is closer than the current best point then we
      // need to search down the other side of the tree.
      if (
        otherChild !== null &&
        this.metric(pointOnHyperplane, node.obj) < bestNode.distance
      ) {
        nearestSearch(otherChild);
      }
    };

    if (this.root) {
      nearestSearch(this.root);
    }

    return bestNode.node!.obj;
  }
}
