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
  TaskStatus,
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
    for (const taskName in taskData) {
      taskData[taskName].sort((a: TaskSummary, b: TaskSummary) => b.attempt - a.attempt);
    }

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
        if (depth[dep] == undefined) {
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
    const taskSpecWidth = [];
    for (let col = 0; col < cols.length; col++) {
      // Get the minimum width of a task spec block needed to fit the entire
      // task spec name.
      let maxWidth = Math.max(botLinkTextWidth, taskLinkTextWidth);
      for (let i = 0; i < cols[col].length; i++) {
        const oldFont = ctx.font;
        const text = cols[col][i].name;
        if (text == selectedTask?.taskKey?.name) {
          ctx.font = "bold " + ctx.font;
        }
        const textWidth = ctx.measureText(text).width + 2 * textMarginX;
        ctx.font = oldFont;
        if (textWidth > maxWidth) {
          maxWidth = textWidth;
        }

        const numTasks = cols[col][i].tasks.length || 1;
        const tasksWidth = taskMarginX + numTasks * (taskWidth + taskMarginX);
        if (tasksWidth > maxWidth) {
          maxWidth = tasksWidth;
        }
      }

      taskSpecWidth.push(maxWidth);
    }

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
    for (let col = 0; col < cols.length; col++) {
      let curY = taskMarginY;
      // Add an entry for each task.
      for (let i = 0; i < cols[col].length; i++) {
        const taskSpec = cols[col][i];
        const entry: taskSpecRect = {
          x: curX,
          y: curY,
          width: taskSpecWidth[col],
          height: taskSpecHeight,
          name: taskSpec.name,
          numTasks: taskSpec.tasks.length,
        };
        taskSpecs.push(entry);
        byName[taskSpec.name] = entry;

        const taskX = curX + taskMarginX;
        const taskY = curY + textHeight + botLinkHeight + taskLinkHeight;
        for (let j = 0; j < taskSpec.tasks.length; j++) {
          const task = taskSpec.tasks[j];
          tasks.push({
            x: taskX + j * (taskWidth + taskMarginX),
            y: taskY,
            width: taskWidth,
            height: taskHeight,
            task: task,
          });
        }
        curY += taskSpecHeight + taskSpecMarginY;
      }
      if (curY > totalHeight) {
        totalHeight = curY;
      }
      curX += taskSpecWidth[col] + taskSpecMarginX;
    }

    totalWidth = curX;

    // Compute the arrows.
    const arrows = [];
    for (const name in graph) {
      const dst = byName[name];
      const deps = graph[name];
      if (deps) {
        for (let j = 0; j < deps.length; j++) {
          const src = byName[deps[j]]
          if (!src) {
            console.log("Error: task " + dst.name + " has unknown parent " + deps[j]);
          } else {
            arrows.push([src, dst]);
          }
        }
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
    console.log(tasks[0]);
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
        ${arrows.map((arrow) => {
          // Start and end points.
          const x1 = arrow[0].x + arrow[0].width;
          const y1 = arrow[0].y + arrow[0].height / 2;
          const x2 = arrow[1].x - arrowWidth;
          const y2 = arrow[1].y + arrow[1].height / 2;
          // Control points.
          const cx1 = x1 + taskSpecMarginX - arrowWidth/2;
          const cy1 = y1;
          const cx2 = x2 - taskSpecMarginX + arrowWidth/2;
          const cy2 = y2;
          const path = ("M"  + x1  + " " + y1
                      + " C" + cx1 + " " + cy1
                      + " "  + cx2 + " " + cy2
                      + " "  + x2  + " " + y2);
          return svg`
            <path
                class="arrow"
                stroke="black"
                stroke-width="1"
                fill="transparent"
                marker-end="url(#arrowhead)"
                d="${path}"
                >
            </path>
          `;
        })}
        ${taskSpecs.map((taskSpec) => svg`
          <rect
              class="taskSpec"
              rx="4"
              ry="4"
              x="${taskSpec.x}"
              y="${taskSpec.y}"
              width="${taskSpec.width}"
              height="${taskSpec.height}"
              style="stroke: black; fill: white; ${taskSpec.name == selectedTask?.taskKey?.name ? "stroke-width: 3px;" : ""}"
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
          <a class="bots" target="_blank" href="${this.computeBotsLink(taskDims[taskSpec.name], swarmingServer)}">
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
          <a class="taskLinks" target="_blank" href="${this.computeTasksLink(taskSpec.name, swarmingServer)}">
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
              href="${this.computeTaskLink(task.task!, swarmingServer)}"
              >
            <rect
                class="task"
                rx="4"
                ry="4" 
                x="${task.x}"
                y="${task.y}"
                width="${task.width}"
                height="${task.height}"
                style="stroke: black; fill: ${taskStatusToTextColor[task.task!.status]}; ${task.task?.id == selectedTask?.id ? "stroke-width: 3px;" : ""}">
            </rect>
          </a>
        `)}
      </svg>
    `, this);
  }

  private computeBotsLink(dims: string[], swarmingServer: string): string {
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

  private computeTaskLink(task: TaskSummary, swarmingServer: string): string {
    const swarmingLink = "https://" + swarmingServer + "/task?id=" + task.swarmingTaskId;
    return "https://task-driver.skia.org/td/" + task.id + "?ifNotFound=" + encodeURIComponent(swarmingLink);
  }

  private computeTasksLink(name: string, swarmingServer: string): string {
    return "https://" + swarmingServer + "/tasklist?f=sk_name-tag:" + name;
  }
}

define('task-graph-sk', TaskGraphSk);
