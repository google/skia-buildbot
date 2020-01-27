/** @module kd
 * A k-d tree implementation.
 * https://en.wikipedia.org/wiki/K-d_tree
 *
 * Forked from https://github.com/Pandinosaurus/kd-tree-javascript
 *
 * https://github.com/ubilabs/kd-tree-javascript
 *
 * @author Mircea Pricop <pricop@ubilabs.net>, 2012
 * @author Martin Kleppe <kleppe@ubilabs.net>, 2012
 * @author Ubilabs http://ubilabs.net, 2012
 * @license MIT License <http://www.opensource.org/licenses/mit-license.php>
 */

/** @class A single node in the k-d Tree. */
class Node {
    constructor(obj, dimension, parent) {
        this.obj = obj;
        this.left = null;
        this.right = null;
        this.parent = parent;
        this.dimension = dimension;
    }
}

/**
 * @class The k-d tree.
 */
export class kdTree {
    /**
     * The constructor.
     *
     * @param {Array} points - An array of {x:x, y:y}.
     * @param {function} metric - A function that calculates the distance
     * between two points.
     * @param {Array} dimensions - The dimensions to use in our points, for
     * example ["x", "y"].
     */
    constructor(points, metric, dimensions) {
        this.dimensions = dimensions;
        this.metric = metric;
        this.root = this.buildTree(points, 0, null);
    }

    /**
     * Builds the from parent Node on down.
     *
     * @param {Array} points - An array of {x:x, y:y}.
     * @param {Number} depth - The current depth from the root node.
     * @param {Node} parent - The parent Node.
     */
    buildTree(points, depth, parent) {
        var dim = depth % this.dimensions.length,
            median,
            node;

        if (points.length === 0) {
            return null;
        }
        if (points.length === 1) {
            return new Node(points[0], dim, parent);
        }

        points.sort((a, b) => {
            return a[this.dimensions[dim]] - b[this.dimensions[dim]];
        });

        median = Math.floor(points.length / 2);
        node = new Node(points[median], dim, parent);
        node.left = this.buildTree(points.slice(0, median), depth + 1, node);
        node.right = this.buildTree(points.slice(median + 1), depth + 1, node);

        return node;
    }

    /**
     * Find the nearest Node to the given point.
     *
     * @param {Object} point - {x:x, y:y}
     */
    nearest(point) {
        var i;
        var bestNode = {
            node: this.root,
            distance: Number.MAX_VALUE,
        };

        const nearestSearch = (node) => {
            var bestChild,
                dimension = this.dimensions[node.dimension],
                ownDistance = this.metric(point, node.obj),
                linearPoint = {},
                linearDistance,
                otherChild,
                i;

            function saveNode(node, distance) {
                bestNode = {
                    node: node,
                    distance: distance,
                };
            }

            for (i = 0; i < this.dimensions.length; i += 1) {
                if (i === node.dimension) {
                    linearPoint[this.dimensions[i]] = point[this.dimensions[i]];
                } else {
                    linearPoint[this.dimensions[i]] = node.obj[this.dimensions[i]];
                }
            }

            linearDistance = this.metric(linearPoint, node.obj);

            if (node.right === null && node.left === null) {
                if (ownDistance < bestNode.distance) {
                    saveNode(node, ownDistance);
                }
                return;
            }

            if (node.right === null) {
                bestChild = node.left;
            } else if (node.left === null) {
                bestChild = node.right;
            } else {
                if (point[dimension] < node.obj[dimension]) {
                    bestChild = node.left;
                } else {
                    bestChild = node.right;
                }
            }

            nearestSearch(bestChild);

            if (ownDistance < bestNode.distance) {
                saveNode(node, ownDistance);
            }

            if (Math.abs(linearDistance) < bestNode.distance) {
                if (bestChild === node.left) {
                    otherChild = node.right;
                } else {
                    otherChild = node.left;
                }
                if (otherChild !== null) {
                    nearestSearch(otherChild);
                }
            }
        }

        if (this.root) {
            nearestSearch(this.root);
        }

        return bestNode.node.obj;
    };
}