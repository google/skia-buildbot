/**
 * @module task_scheduler/modules/task-graph-sk
 * @description <h2><code>task-graph-sk</code></h2>
 *
 * <p>
 * This element displays a graph which shows the relationship between a set of
 * tasks.
 * </p>
 */

import { html, render, svg, TemplateResult } from 'lit-html';
import { define } from 'elements-sk/define'

import {
  Job,
  Task,
  TaskDependencies,
  TaskDimensions,
  TaskSummaries,
  TaskSummary,
} from '../rpc';


export class TaskGraphSk extends HTMLElement {
  draw(jobs: Job[], swarmingServer: string, selectedTask?: Task) {
    const graph: {[key:string]:string[]} = {};
    const taskData: {[key:string]:TaskSummary[]} = {};
    const taskDims: {[key:string]:string[]} = {};
    jobs.forEach((job: Job) => {
      job.dependencies?.forEach((dep: TaskDependencies) => {
        if (dep.dependencies && !graph[dep.task]) {
          graph[dep.task] = dep.dependencies;
        }
      });
      job.taskDimensions?.forEach((dims: TaskDimensions) => {
        if (!taskDims[dims.taskName]) {
          taskDims[dims.taskName] = dims.dimensions!;
        }
      });
      job.tasks?.forEach((taskSummaries: TaskSummaries) => {
        const tasks = taskData[taskSummaries.name] || [];
        taskSummaries.tasks?.forEach((task: TaskSummary) => {
          if (!tasks.find((item) => item.id === task.id)) {
            tasks.push(task);
          }
        });
        taskData[taskSummaries.name] = tasks;
      });
    });

    // Sort the tasks and task specs for consistency.
    const graphKeys = Object.keys(graph).sort();
    Object.values(graph).forEach((tasks: string[]) => {tasks.sort()});
    Object.values(taskData).forEach((tasks: TaskSummary[]) => {
      tasks.sort((a: TaskSummary, b: TaskSummary) => b.attempt - a.attempt);
    });

    // Skip drawing the graph if taskData or graph are missing or empty. This
    // is mainly to prevent errors on the demo page.
    if (!taskData || !graph || Object.keys(taskData).length == 0 || Object.keys(graph).length == 0) {
      console.log("Not drawing graph; taskData or graph not ready.");
      return;
    }
    console.log("Drawing tasks graph.");

    // Compute the "depth" of each task spec.
    class cell {
      name: string = "";
      tasks: TaskSummary[] = [];
    }
    const depth: {[key:string]: number} = {};
    const cols: cell[][] = [];
    const visited: {[key:string]: boolean} = {};

    const visit = function(current: string) {
      visited[current] = true
      let myDepth = 0;
      (graph[current] || []).forEach((dep: string) => {
        // Visit the dep if we haven't yet. Its depth may be zero, so we have
        // to explicitly use "depth[dep] == undefined" instead of "!depth[dep]"
        if (!visited[dep]) {
          visit(dep);
        }
        if (depth[dep] >= myDepth) {
          myDepth = depth[dep] + 1;
        }
      });
      depth[current] = myDepth;
      if (cols.length == myDepth) {
        cols.push([]);
      } else if (myDepth > cols.length) {
        console.log("_computeTasksGraph skipped a column!");
        return;
      }
      cols[myDepth].push({
          name: current,
          tasks: taskData[current] || [],
      });
    };

    // Visit all of the nodes.
    graphKeys.forEach((key: string) => {
      if (!visited[key]) {
        visit(key);
      }
    });

    const arrowWidth = 4;
    const arrowHeight = 4;
    const botLinkFontSize = 11;
    const botLinkMarginX = 10;
    const botLinkMarginY = 4;
    const botLinkHeight = botLinkFontSize + 2*botLinkMarginY;
    const botLinkText = "view swarming bots";
    const fontFamily = "Arial";
    const fontSize = 12;
    const taskSpecMarginX = 20;
    const taskSpecMarginY = 20;
    const taskMarginX = 10;
    const taskMarginY = 10;
    const textMarginX = 10;
    const textMarginY = 10;
    const taskWidth = 30;
    const taskHeight = 30;
    const taskLinkFontSize = botLinkFontSize;
    const taskLinkMarginX = botLinkMarginX;
    const taskLinkMarginY = botLinkMarginY;
    const taskLinkHeight = taskLinkFontSize + 2*taskLinkMarginY;
    const taskLinkText = "view swarming tasks";
    const textOffsetX = textMarginX;
    const textOffsetY = fontSize + textMarginY;
    const textHeight = fontSize + 2 * textMarginY;
    const botLinkOffsetY = textOffsetY + botLinkFontSize + botLinkMarginY;
    const taskLinkOffsetY = botLinkOffsetY + taskLinkFontSize + taskLinkMarginY;
    const taskSpecHeight = textHeight + botLinkHeight + taskLinkHeight + taskHeight + taskMarginY;

    // Compute the task spec block width for each column.
    const maxTextWidth = 0;
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d")!;
    ctx.font = botLinkFontSize + "px " + fontFamily;
    const botLinkTextWidth = ctx.measureText(botLinkText).width + 2 * botLinkMarginX;
    ctx.font = taskLinkFontSize + "px " + fontFamily;
    const taskLinkTextWidth = ctx.measureText(taskLinkText).width + 2 * taskLinkMarginX;
    ctx.font = fontSize + "px " + fontFamily;
    const taskSpecWidth: number[] = [];
    cols.forEach((col: cell[]) => {
      // Get the minimum width of a task spec block needed to fit the entire
      // task spec name.
      let maxWidth = Math.max(botLinkTextWidth, taskLinkTextWidth);
      for (let i = 0; i < col.length; i++) {
        const oldFont = ctx.font;
        const text = col[i].name;
        if (text == selectedTask?.taskKey?.name) {
          ctx.font = "bold " + ctx.font;
        }
        const textWidth = ctx.measureText(text).width + 2 * textMarginX;
        ctx.font = oldFont;
        if (textWidth > maxWidth) {
          maxWidth = textWidth;
        }

        const numTasks = col[i].tasks.length || 1;
        const tasksWidth = taskMarginX + numTasks * (taskWidth + taskMarginX);
        if (tasksWidth > maxWidth) {
          maxWidth = tasksWidth;
        }
      }
      taskSpecWidth.push(maxWidth);
    })

    // Lay out the task specs and tasks.
    class taskSpecRect {
      x: number = 0;
      y: number = 0;
      width: number = 0;
      height: number = 0;
      name: string = "";
      numTasks: number = 0;
    }
    class taskRect {
      x: number = 0;
      y: number = 0;
      width: number = 0;
      height: number = 0;
      task: TaskSummary | null = null;
    }
    let totalWidth = 0;
    let totalHeight = 0;
    const taskSpecs: taskSpecRect[] = [];
    const tasks: taskRect[] = [];
    const byName: {[key:string]: taskSpecRect} = {};
    let curX = taskMarginX;
    cols.forEach((col: cell[], colIdx: number) => {
      let curY = taskMarginY;
      // Add an entry for each task.
      col.forEach((taskSpec: cell) => {
        const entry: taskSpecRect = {
          x: curX,
          y: curY,
          width: taskSpecWidth[colIdx],
          height: taskSpecHeight,
          name: taskSpec.name,
          numTasks: taskSpec.tasks.length,
        };
        taskSpecs.push(entry);
        byName[taskSpec.name] = entry;

        const taskX = curX + taskMarginX;
        const taskY = curY + textHeight + botLinkHeight + taskLinkHeight;
        taskSpec.tasks.forEach((task: TaskSummary, taskIdx: number) => {
          tasks.push({
            x: taskX + taskIdx * (taskWidth + taskMarginX),
            y: taskY,
            width: taskWidth,
            height: taskHeight,
            task: task,
          });
        });
        curY += taskSpecHeight + taskSpecMarginY;
      });
      if (curY > totalHeight) {
        totalHeight = curY;
      }
      curX += taskSpecWidth[colIdx] + taskSpecMarginX;
    });

    totalWidth = curX;

    // Compute the arrows.
    let arrows: string[] = [];
    for (const name in graph) {
      const dst = byName[name];
      const deps = graph[name];
      if (deps) {
        deps.forEach((dep: string) => {
          const src = byName[dep]
          if (!src) {
            console.log("Error: task " + dst.name + " has unknown parent " + dep);
            return "";
          } else {
            // Start and end points.
            const x1 = src.x + src.width;
            const y1 = src.y + src.height / 2;
            const x2 = dst.x - arrowWidth;
            const y2 = dst.y + dst.height / 2;
            // Control points.
            const cx1 = x1 + taskSpecMarginX - arrowWidth/2;
            const cy1 = y1;
            const cx2 = x2 - taskSpecMarginX + arrowWidth/2;
            const cy2 = y2;
            arrows.push("M"  + x1  + " " + y1
                      + " C" + cx1 + " " + cy1
                      + " "  + cx2 + " " + cy2
                      + " "  + x2  + " " + y2);
          }
        });
      }
    }

    const taskStatusToTextColor: {[key:string]: string} = {
      TASK_STATUS_PENDING: "rgb(255, 255, 255)",
      TASK_STATUS_RUNNING: "rgb(248, 230, 180)",
      TASK_STATUS_SUCCESS: "rgb(209, 228, 188)",
      TASK_STATUS_FAILURE: "rgb(217, 95, 2)",
      TASK_STATUS_MISHAP:  "rgb(117, 112, 179)",
    };

    // Draw the graph.
    render(svg`
      <svg width="${totalWidth}" height="${totalHeight}">
        <marker
            id="arrowhead"
            class="arrowhead"
            viewBox="0 0 10 10"
            refX="0"
            refY="5"
            markerUnits="strokeWidth"
            markerWidth="${arrowWidth}"
            markerHeight="${arrowHeight}"
            orient="auto"
            >
          <path d="M 0 0 L 10 5 L 0 10 Z"></path>
        </marker>
        ${arrows.map((arrow: string) => {
          return svg`
            <path
                class="arrow"
                stroke="black"
                stroke-width="1"
                fill="transparent"
                marker-end="url(#arrowhead)"
                d="${arrow}"
                >
            </path>
          `;
        })}
        ${taskSpecs.map((taskSpec: taskSpecRect) => svg`
          <rect
              class="taskSpec"
              rx="4"
              ry="4"
              x="${taskSpec.x}"
              y="${taskSpec.y}"
              width="${taskSpec.width}"
              height="${taskSpec.height}"
              style="stroke: black; fill: white; ${
                  taskSpec.name == selectedTask?.taskKey?.name ? "stroke-width: 3px;" : ""
              }"
              >
          </rect>
          <text
              class="taskSpec"
              font-family="${fontFamily}"
              font-size="${fontSize}"
              x="${taskSpec.x + textOffsetX}"
              y="${taskSpec.y + textOffsetY}"
              font-weight="${taskSpec.name == selectedTask?.taskKey?.name ? "bold" : "normal"}"
              >
            ${taskSpec.name}
          </text>
          <a
              class="bots"
              target="_blank"
              href="${TaskGraphSk.computeBotsLink(taskDims[taskSpec.name], swarmingServer)}">
            <text
                class="bots"
                font-family="${fontFamily}"
                font-size="${fontSize}"
                style="text-decoration: underline;"
                x="${taskSpec.x + textOffsetX}"
                y="${taskSpec.y + botLinkOffsetY}"
                >
              ${botLinkText}
            </text>
          </a>
          <a
              class="taskLinks"
              target="_blank"
              href="${TaskGraphSk.computeTasksLink(taskSpec.name, swarmingServer)}">
            <text
                class="taskLinks"
                font-family="${fontFamily}"
                font-size="${taskLinkFontSize}"
                style="text-decoration: underline;"
                x="${taskSpec.x + textOffsetX}"
                y="${taskSpec.y + taskLinkOffsetY}"
                >
              ${botLinkText}
            </text>
          </a>
        `)}
        ${tasks.map((task) => svg`
          <a
              class="task"
              target="_blank"
              href="${TaskGraphSk.computeTaskLink(task.task!, swarmingServer)}">
            <rect
                class="task"
                rx="4"
                ry="4" 
                x="${task.x}"
                y="${task.y}"
                width="${task.width}"
                height="${task.height}"
                style="stroke: black; fill: ${taskStatusToTextColor[task.task!.status]};
                    ${task.task?.id == selectedTask?.id ? "stroke-width: 3px;" : ""}">
            </rect>
          </a>
        `)}
      </svg>
    `, this);
  }

  private static computeBotsLink(dims: string[], swarmingServer: string): string {
    let link = "https://" + swarmingServer + "/botlist";
    if (dims) {
      for (let i = 0; i < dims.length; i++) {
        if (i == 0) {
          link += "?";
        } else {
          link += "&";
        }
        link += "f=" + encodeURIComponent(dims[i]);
      }
    }
    return link;
  }

  private static computeTaskLink(task: TaskSummary, swarmingServer: string): string {
    const swarmingLink = "https://" + swarmingServer + "/task?id=" + task.swarmingTaskId;
    return "https://task-driver.skia.org/td/" + task.id + "?ifNotFound=" +
        encodeURIComponent(swarmingLink);
  }

  private static computeTasksLink(name: string, swarmingServer: string): string {
    return "https://" + swarmingServer + "/tasklist?f=sk_name-tag:" + name;
  }
}

define('task-graph-sk', TaskGraphSk);
