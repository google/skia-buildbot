/**
 * @module modules/branches-sk
 * @description <h2><code>branches-sk</code></h2>
 *
 *  Custom element for displaying branches.
 */
import { define } from 'elements-sk/define';
import { $$ } from 'common-sk/modules/dom';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { truncate } from '../../../infra-sk/modules/string';
import { Commit } from '../commits-table-sk/commits-table-sk';
import { AutorollerStatus, Branch } from '../rpc';

const MIN_CANVAS_WIDTH = 175;
const commitY = 20; // Vertical pixels used by each commit.
const paddingX = 10; // Left-side padding pixels.
const paddingY = 20; // Top padding pixels.
const radius = 3; // Radius of commit dots.
const columnWidth = commitY; // Pixel width of per-branch colums.
const commitBg = '#FFFFFF'; // Background color of alternating commits.
const commitBgAlt = '#EFEFEF'; // Background color of alternating commits.
const font = '10px monospace'; // Font used for labels.
// This is filled in later.
let palette: Array<string> = [];

const BRANCH_PREFIX = 'origin/';

interface CommitInfo {
  hash: string;
  timestamp?: string;
  parents?: Array<string>;
}

class Point {
  constructor(x: number, y: number) {
    this.x = x;
    this.y = y;
  }
  x: number;
  y: number;
}

class DisplayCommit {
  constructor(commit: CommitInfo, row: number) {
    this.hash = commit.hash;
    this.timestamp = new Date(commit.timestamp!);
    this.row = row;
    this.column = -1;
    this.label = [];
    this.parents = commit.parents || [];
    this.children = [];
  }
  hash: string;
  timestamp: Date;
  row: number;
  column: number;
  label: Array<string>;
  parents: Array<string>;
  children: Array<string>;
  color() {
    return palette[this.column % palette.length];
  }
  // Where to draw this commit.
  getBounds() {
    return new Point(paddingX, paddingY - commitY / 4 + commitY * this.row);
  }

  // The center of this commit's dot.
  dotCenter() {
    var start = this.getBounds();
    var centerX = start.x + columnWidth * this.column + radius;
    var centerY = start.y - radius - 2;
    return new Point(centerX, centerY);
  }

  // Coordinates for drawing this commit's label.
  labelCoords() {
    var bounds = this.getBounds();
    var center = this.dotCenter();
    return new Point(center.x + 3 * radius, bounds.y - 1);
  }

  // Return the text for this commit's label, truncated to 24 characters.
  labelText() {
    return truncate(this.label.join(','), 24);
  }

  // Return the estimated width of this commit's label text.
  labelWidth(ctx: CanvasRenderingContext2D) {
    return ctx.measureText(this.labelText()).width;
  }

  // Draw an an alternating background color for this commit.
  drawBackground(ctx: CanvasRenderingContext2D) {
    var startY = commitY * this.row;
    var bgColor = this.row % 2 ? commitBg : commitBgAlt;
    ctx.fillStyle = bgColor;
    ctx.fillRect(0, startY, ctx.canvas.clientWidth, startY + commitY);
  }
  // Draw a line connecting this commit to one of its parents.
  drawConnection(
    ctx: CanvasRenderingContext2D,
    parent: DisplayCommit,
    allCommits: Map<string, DisplayCommit>
  ) {
    const center = this.dotCenter();
    const to = parent.dotCenter();
    ctx.beginPath();
    ctx.moveTo(center.x, center.y);
    if (this.column == parent.column) {
      // Draw a straight line.
      ctx.lineTo(to.x, to.y);
    } else {
      // Draw a connector composed of five segments: a vertical line, an
      // arc, a horizontal line, another arc, and another vertical line.
      // One or more of the lines may have zero length.
      const arcRadius = commitY / 2;
      // The direction in which to draw the arc.
      const d = center.x > to.x ? 1 : -1;

      // We'll reuse these values, so pre-compute them.
      const halfPI = 0.5 * Math.PI;
      const oneAndHalfPI = 1.5 * Math.PI;

      // If there is at least one commit in the current commit's column
      // between the current commit and this parent, the first arc must
      // begin at the current commit: the first vertical line has zero
      // length. Otherwise, the length of the first vertical line is
      // flexible.
      let v1_flex = true;
      for (const parentHash of this.parents) {
        var c = allCommits.get(parentHash);
        if (!c) {
          console.warn('Cannot find ' + parentHash);
          continue;
        }
        if (this.timestamp > c.timestamp && c.timestamp > parent.timestamp) {
          if (this.column == c.column) {
            v1_flex = false;
            break;
          }
        }
      }

      // If there is at least one commit in the parent's column between the
      // current commit and this parent, the second arc must end at the
      // parent commit: the second vertical line has zero length.
      // Otherwise, the length of the second vertical line is flexible.
      var v2_flex = true;
      for (const childHash of parent.children) {
        let c = allCommits.get(childHash)!;
        if (this.timestamp > c.timestamp && c.timestamp > parent.timestamp) {
          if (parent.column == c.column) {
            v2_flex = false;
            break;
          }
        }
      }

      // Arc information..
      var a1 = new Point(center.x - d * arcRadius, to.y - commitY);
      var a2 = new Point(to.x + d * arcRadius, to.y);

      // If both vertical lines are flexible, arbitrarily choose where to
      // put the arcs and horizontal line (eg. next to the parent).
      if (v1_flex && v2_flex) {
        a1.y = to.y - commitY;
        a2.y = to.y;
      }
      // If exactly one vertical line is flexible, put the arcs and
      // horizontal line where they must go.
      else if (v1_flex && !v2_flex) {
        a1.y = to.y - commitY;
        a2.y = to.y;
      } else if (!v1_flex && v2_flex) {
        a1.y = center.y;
        a2.y = center.y + commitY;
      }
      // If neither vertical line is flexible, then we have to place arcs
      // at both commits and the "horizontal" line becomes diagonal.
      else {
        a1.y = center.y;
        a2.y = to.y;
      }

      // Distance between the two arc centers.
      var dist = Math.sqrt(Math.pow(a2.x - a1.x, 2) + Math.pow(a2.y - a1.y, 2));
      // Length of the arc to draw.
      var arcLength =
        Math.PI -
        Math.acos((2 * arcRadius) / dist) -
        Math.acos((Math.abs(to.x - center.x) - 2 * arcRadius) / dist);
      var a1_start = halfPI - d * halfPI;
      var a2_start = oneAndHalfPI - d * (halfPI - arcLength);

      // Draw the connector: vertical line, arc, horizontal line, arc,
      // vertical line.
      ctx.lineTo(a1.x + d * arcRadius, a1.y);
      ctx.arc(a1.x, a1.y, arcRadius, a1_start, a1_start + d * arcLength, d < 0);
      // The middle line doesn't need to be explicitly drawn.
      ctx.arc(a2.x, a2.y, arcRadius, a2_start, a2_start - d * arcLength, d > 0);
      ctx.lineTo(to.x, to.y);
    }
    ctx.strokeStyle = this.color();
    ctx.stroke();
  }

  // Draw this commit's label.
  drawLabel(ctx: CanvasRenderingContext2D) {
    if (this.label.length <= 0) {
      return;
    }
    const labelCoords = this.labelCoords();
    var w = this.labelWidth(ctx);
    var h = parseInt(font);
    var paddingY = 3;
    var paddingX = 3;
    ctx.fillStyle = this.color();
    ctx.fillRect(labelCoords.x - paddingX, labelCoords.y - h, w + 2 * paddingX, h + paddingY);
    ctx.fillStyle = '#FFFFFF';
    ctx.fillText(this.labelText(), labelCoords.x, labelCoords.y);
  }

  draw(ctx: CanvasRenderingContext2D, displayCommits: Map<string, DisplayCommit>) {
    const color = this.color();
    const center = this.dotCenter();

    // Connect the dots.
    for (let parentHash of this.parents) {
      const parent = displayCommits.get(parentHash);
      if (!parent) {
        console.warn('Cannot find ' + parentHash);
        continue;
      }
      this.drawConnection(ctx, parent, displayCommits);
    }

    // Draw a dot.
    drawDot(ctx, center, radius, color);

    // Draw a label, if applicable.
    this.drawLabel(ctx);
  }
}

export class BranchesSk extends ElementSk {
  private _repoUrl: string = '';
  private _commits: Array<Commit> = [];
  private _branchHeads: Array<Branch> = [];
  private _rolls: Array<AutorollerStatus> = [];
  // Artificial 'Branches' we use to label autorollers, derived from the above.
  private rollLabels: Array<Branch> = [];
  private displayCommits: Map<string, DisplayCommit> = new Map();
  private canvasWidth: number = 0;
  private canvasHeight: number = 0;
  // Map of commit index -> link to branch  or roller.
  private linkMap: Map<number, string> = new Map();
  // Map of commit index -> title containing branches and rollers.
  private titleMap: Map<number, string> = new Map();
  private canvas?: HTMLCanvasElement;

  private static template = (el: BranchesSk) =>
    html`
      <!-- The tap event (which was originally used) does not always produce offsetY.
      on-click works for the Pixels (even when touching), so we use that.-->
      <canvas
        id="commitCanvas"
        @click=${(e: MouseEvent) => el.handleClick(e)}
        @mousemove=${(e: MouseEvent) => el.handleMousemove(e)}
      ></canvas>
    `;

  constructor() {
    super(BranchesSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('theme-chooser-toggle', this.draw);
    this._render();
    this.canvas = $$<HTMLCanvasElement>('#commitCanvas', this)!;
    this.draw();
  }
  disconnectedCallback() {
    document.removeEventListener('theme-chooser-toggle', this.draw);
  }

  get commits(): Array<Commit> {
    return this._commits;
  }

  set commits(value: Array<Commit>) {
    this._commits = value;
    this.draw();
  }

  get branchHeads(): Array<Branch> {
    return this._branchHeads;
  }

  set branchHeads(value: Array<Branch>) {
    this._branchHeads = value;
    this.draw();
  }

  get rolls(): Array<AutorollerStatus> {
    return this._rolls;
  }

  set rolls(value) {
    this._rolls = value;
    this.rollLabels = [
      ...this.rolls.map((roll) => {
        return { name: roll.name + ' rolled', head: roll.lastRollRev };
      }),
      ...this.rolls.map((roll) => {
        return { name: roll.name + ' rolling', head: roll.currentRollRev };
      }),
    ];
  }
  get repoUrl(): string {
    return this._repoUrl;
  }

  set repoUrl(value) {
    this._repoUrl = value;
  }

  private computeLinkMap() {
    this.linkMap.clear();
    // Link to generic branch heads.
    for (const branch of this.branchHeads) {
      let name = branch.name;
      if (branch.name.startsWith(BRANCH_PREFIX)) {
        name = branch.name.slice(BRANCH_PREFIX.length);
      }
      // If the commit is not found in the range of commits, we will just be
      // overwriting the value at key "-1", which won't actually get used.
      const idx = this._indexOfRevision(branch.head);
      this.linkMap.set(idx, this.repoUrl + name);
    }

    // Link to rolls.
    for (const roller of this.rolls) {
      let idx = this._indexOfRevision(roller.currentRollRev);
      this.linkMap.set(idx, roller.url);
      idx = this._indexOfRevision(roller.lastRollRev);
      this.linkMap.set(idx, roller.url);
    }
  }

  private computeTitleMap() {
    this.titleMap.clear();
    for (const branch of [...this.branchHeads, ...this.rollLabels]) {
      const name = branch.name;
      const idx = this._indexOfRevision(branch.head);
      if (this.titleMap.has(idx)) {
        this.titleMap.set(idx, this.titleMap.get(idx) + ',' + name);
      } else {
        this.titleMap.set(idx, name);
      }
    }
  }

  _indexOfRevision(revision: string) {
    return this.commits.findIndex((c) => c.hash === revision);
  }

  private handleClick(e: MouseEvent) {
    if (!this.linkMap) {
      return;
    }
    const y = (e && e.offsetY) || 1;
    const commitIdx = Math.floor(y / commitY);

    const link = this.linkMap.get(commitIdx);
    if (link) {
      window.open(link);
    }
  }

  private handleMousemove(e: MouseEvent) {
    if (!this.linkMap) {
      return;
    }
    const commitIdx = Math.floor(e.offsetY / commitY);
    const link = this.linkMap.get(commitIdx);
    if (link) {
      this.canvas!.classList.add('pointer');
    } else {
      this.canvas!.classList.remove('pointer');
    }
    const title = this.titleMap.get(commitIdx);
    if (title) {
      this.canvas!.title = title;
    }
  }

  private draw = () => {
    console.time('draw');
    // Initialize all commits.
    this.displayCommits = prepareCommitsForDisplay(this.commits, this.branchHeads, this.rollLabels);

    // Calculate the required canvas width based on the commit columns and
    // labels.
    // TODO(borenet): Further minimize this width by reordering the columns
    // based on which has the longest label.
    let dummyCtx = document.createElement('canvas').getContext('2d')!;
    dummyCtx.font = font;
    let longestWidth = 0;
    for (let commit of this.commits) {
      let c = this.displayCommits.get(commit.hash)!;
      let w = c.labelWidth(dummyCtx);
      w += commitY * (c.column + 1);
      if (w > longestWidth) {
        longestWidth = w;
      }
    }

    // Redraw the canvas.
    const scale = window.devicePixelRatio || 1.0;
    const canvas = this.canvas!;
    this.canvasWidth = Math.max(longestWidth + paddingX, MIN_CANVAS_WIDTH);
    this.canvasHeight = commitY * this.commits.length;
    canvas.style.width = Math.floor(this.canvasWidth) + 'px';
    canvas.style.height = Math.floor(this.canvasHeight) + 'px';
    canvas.width = this.canvasWidth * scale;
    canvas.height = this.canvasHeight * scale;
    const ctx = canvas.getContext('2d') as CanvasRenderingContext2D;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.setTransform(scale, 0, 0, scale, 0, 0);
    ctx.font = font;

    // Create the color palette for the commits.
    palette = [
      getComputedStyle(this).getPropertyValue('--branch-color-0'),
      getComputedStyle(this).getPropertyValue('--branch-color-1'),
      getComputedStyle(this).getPropertyValue('--branch-color-2'),
      getComputedStyle(this).getPropertyValue('--branch-color-3'),
      getComputedStyle(this).getPropertyValue('--branch-color-4'),
    ];

    // Draw the commits.
    for (const commit of this.commits) {
      this.displayCommits.get(commit.hash)!.draw(ctx, this.displayCommits);
    }

    this.computeLinkMap();
    this.computeTitleMap();

    console.timeEnd('draw');
  };
}

// Create Commit objects to be displayed. Assigns rows and columns for each
// commit to assist in producing a nice layout.
function prepareCommitsForDisplay(
  commits: Array<Commit>,
  branch_heads: Array<Branch>,
  rolls: Array<Branch>
): Map<string, DisplayCommit> {
  // Create a Commit object for each commit.
  const displayCommits: Map<string, DisplayCommit> = new Map(); // Commit objects by hash.
  const remaining: Map<string, DisplayCommit> = new Map(); // Not-yet-processed commits by hash.
  for (let i = 0; i < commits.length; i++) {
    const c = new DisplayCommit(commits[i], i);
    displayCommits.set(c.hash, c);
    remaining.set(c.hash, c);
  }

  // Pre-process the branches. We want master first, and no HEAD.
  let masterIdx = -1;
  const branches: Array<Branch> = [];
  for (let b = 0; b < branch_heads.length; b++) {
    if (branch_heads[b].name === 'master') {
      masterIdx = b;
      branches.push(branch_heads[b]);
    }
  }
  for (let b = 0; b < branch_heads.length; b++) {
    var branch = branch_heads[b];
    if (b != masterIdx && branch.name != 'HEAD') {
      branches.push(branch);
    }
  }
  // Add Autoroller labels.
  branches.push(...rolls);

  // Trace each branch, placing commits on that branch in an associated column.
  let column = 0;
  for (let branch of branches) {
    // Add a label to commits at branch heads.
    const hash = branch.head;
    // The branch might have scrolled out of the time window. If so, just
    // skip it.
    if (!displayCommits.has(hash)) {
      continue;
    }
    displayCommits.get(hash)!.label.push(branch.name);
    if (traceCommits(displayCommits, commits, remaining, hash, column)) {
      column++;
    }
  }

  // Add the remaining commits to their own columns.
  for (const hash in remaining) {
    if (traceCommits(displayCommits, commits, remaining, hash, column)) {
      column++;
    }
  }

  // Point all parents at their children, for convenience.
  for (const [_, commit] of displayCommits) {
    for (let parentHash of commit.parents) {
      if (!displayCommits.has(parentHash)) {
        console.warn('Cannot find ' + parentHash);
        continue;
      }
      displayCommits.get(parentHash)!.children.push(commit.hash);
    }
  }

  return displayCommits;
}
// Follow commits by first parent, assigning the given column until we get
// to a commit that we aren't going to draw.
function traceCommits(
  displayCommits: Map<string, DisplayCommit>,
  commits: Array<CommitInfo>,
  remaining: Map<string, DisplayCommit>,
  hash: string,
  column: number
) {
  let usedColumn = false;
  while (remaining.has(hash)) {
    const c = displayCommits.get(hash)!;
    c.column = column;
    remaining.delete(hash);
    hash = c.parents[0];
    usedColumn = true;
    // Special case for non-displayed parents.
    if (!displayCommits.has(hash)) {
      let offscreenParent = new DisplayCommit(
        {
          hash: hash,
        },
        commits.length
      );
      offscreenParent.column = c.column;
      displayCommits.set(hash, offscreenParent);
    }
  }
  return usedColumn;
}

// Draws a filled-in dot at the given center with the given radius and color.
function drawDot(ctx: CanvasRenderingContext2D, center: Point, radius: number, color: string) {
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(center.x, center.y, radius, 0, 2 * Math.PI, false);
  ctx.fill();
  ctx.closePath();
}

define('branches-sk', BranchesSk);
