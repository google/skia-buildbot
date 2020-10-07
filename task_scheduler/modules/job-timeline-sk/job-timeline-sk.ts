/**
 * @module modules/job-timeline-sk
 * @description <h2><code>job-timeline-sk</code></h2>
 *
 * Displays a timeline of the tasks which comprise a given job, to easily
 * determine where time is being spent.
 */
import { define } from 'elements-sk/define';
import { Job, Task } from '../rpc';
import { draw, Block, Lane, Data } from '../gantt-chart-sk';

function ts(tsStr: string) {
  // If the timestamp is zero-ish, return the current datetime.
  if (Date.parse(tsStr) <= 0) {
    return new Date();
  }
  return new Date(tsStr);
}

export class JobTimelineSk extends HTMLElement {
  draw(job: Job, tasks: Task[], epochs: string[]) {
    const lanes: Map<string, Lane> = new Map();
    for (const t of tasks) {
      // Create the lane if it doesn't already exist.
      let lane = lanes.get(t.taskKey!.name);
      if (!lane) {
        lane = {
          label: t.taskKey!.name,
          blocks: [],
        };
        lanes.set(t.taskKey!.name, lane);
      }

      // Creation timestamp may be after start and finish timestamps in the
      // case of deduplicated tasks. Since we care more about the
      // contribution to the job than the task itself, set the start and
      // finish timestamps equal to the creation timestamp in this case.
      const createTs = ts(t.createdAt!);
      if (t.startedAt && ts(t.startedAt!).getTime() < createTs.getTime()) {
        t.startedAt = t.createdAt;
      }
      if (t.finishedAt && ts(t.finishedAt!).getTime() < createTs.getTime()) {
        t.finishedAt = t.createdAt;
      }

      // Create blocks. If we have performance data from Swarming, each task
      // becomes multiple blocks, indicating the download and upload overhead
      // as well as the run time itself.
      let lastBlockEnd = createTs;
      const makeBlock = function(label: string, end: string, color: string) {
        const block: Block = {
          label: label,
          start: lastBlockEnd,
          end: ts(end),
          color: color,
        };
        lastBlockEnd = block.end;
        lane!.blocks.push(block);
      };
      if (t.startedAt) {
        makeBlock("pending", t.startedAt, "#e69f00");
      }
      if (t.stats) {
        const startTs = ts(t.startedAt!).getTime();
        const finishTs = ts(t.finishedAt!).getTime();
        makeBlock("overhead", new Date(startTs + 1000*parseFloat(t.stats.downloadOverheadS)).toString(), "#d55e00");
        makeBlock("running", new Date(finishTs - 1000*parseFloat(t.stats.uploadOverheadS)).toString(), "#0072b2");
        makeBlock("overhead", t.finishedAt!, "#d55e00");
      } else {
        makeBlock("running", t.finishedAt || "", "#0072b2");
      }
    }

    // Draw the chart.
    const data: Data = {
      lanes: Array.from(lanes.values()),
      epochs: epochs.map((epoch) => ts(epoch)),
    };
    if (job.requestedAt) {
      data.start = ts(job.requestedAt);
    }
    if (job.finishedAt) {
      data.end = ts(job.finishedAt);
    }
    draw(this, data);
  }
};

define('job-timeline-sk', JobTimelineSk);
