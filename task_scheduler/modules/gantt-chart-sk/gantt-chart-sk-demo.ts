import './index';
import { draw, Data } from './gantt-chart-sk';

// This container holds the active chart, if any.
let container: HTMLDivElement | null = null;

// clearAndDraw is a helper for drawing charts.
const clearAndDraw = (data: Data) => {
  // Delete and recreate the container div so that we render from scratch.
  container?.remove();
  container = document.createElement("div");
  container.id = "container";
  container.style.height = "500px";
  container.style.width = "1000px";
  document.body.appendChild(container);

  // Draw the new chart.
  draw(container, data);
}

// getData returns some placeholder data for a chart.
const getData = (): Data => {
  return {
    lanes: [
      {
        label: "lane1",
        blocks: [
          {
            start: new Date("1970-01-19T12:54:10.000Z"),
            end:   new Date("1970-01-19T12:54:20.000Z"),
            color: "blue",
            label: "block1",
          },
        ],
      },
      {
        label: "lane2",
        blocks: [
          {
            start: new Date("1970-01-19T12:54:11.000Z"),
            end:   new Date("1970-01-19T12:54:12.000Z"),
            color: "red",
            label: "block1",
          },
          {
            start: new Date("1970-01-19T12:54:13.000Z"),
            end:   new Date("1970-01-19T12:54:14.000Z"),
            color: "green",
            label: "block2",
          },
        ],
      },
    ],
    start: undefined,
    end: undefined,
    epochs: undefined,
  };
}

// Set up event handlers for the buttons to render charts.

document.getElementById("simple")!.addEventListener("click", (ev: MouseEvent) => {
  clearAndDraw(getData());
});

document.getElementById("simple-start-end")!.addEventListener("click", (ev: MouseEvent) => {
  const data = getData();
  data.start = new Date("1970-01-19T12:54:05.000Z");
  data.end = new Date("1970-01-19T12:54:25.000Z");
  clearAndDraw(data);
});

document.getElementById("simple-epochs")!.addEventListener("click", (ev: MouseEvent) => {
  const data = getData();
  data.epochs = [
    new Date("1970-01-19T12:54:10.000Z"),
    new Date("1970-01-19T12:54:11.000Z"),
    new Date("1970-01-19T12:54:12.000Z"),
    new Date("1970-01-19T12:54:13.000Z"),
    new Date("1970-01-19T12:54:14.000Z"),
    new Date("1970-01-19T12:54:15.000Z"),
    new Date("1970-01-19T12:54:16.000Z"),
    new Date("1970-01-19T12:54:17.000Z"),
    new Date("1970-01-19T12:54:18.000Z"),
    new Date("1970-01-19T12:54:19.000Z"),
    new Date("1970-01-19T12:54:20.000Z"),
  ];
  clearAndDraw(data);
});
