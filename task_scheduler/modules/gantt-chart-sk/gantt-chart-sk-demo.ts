import './index';
import { draw, Data } from './gantt-chart-sk';

// clearAndDraw is a helper for drawing charts.
const clearAndDraw = (data: Data) => {
  // Delete and recreate the container div so that we render from scratch.
  let container = <HTMLDivElement>document.getElementById("container")!;
  if (container) {
    document.body.removeChild(container);
  }
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
            start: new Date(1601650000),
            end:   new Date(1601660000),
            color: "blue",
            label: "block1",
          },
        ],
      },
      {
        label: "lane2",
        blocks: [
          {
            start: new Date(1601651000),
            end:   new Date(1601652000),
            color: "red",
            label: "block1",
          },
          {
            start: new Date(1601653000),
            end:   new Date(1601654000),
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
  data.start = new Date(1601645000);
  data.end = new Date(1601665000);
  clearAndDraw(data);
});

document.getElementById("simple-epochs")!.addEventListener("click", (ev: MouseEvent) => {
  const data = getData();
  data.epochs = [
    new Date(1601650000),
    new Date(1601651000),
    new Date(1601652000),
    new Date(1601653000),
    new Date(1601654000),
    new Date(1601655000),
    new Date(1601656000),
    new Date(1601657000),
    new Date(1601658000),
    new Date(1601659000),
    new Date(1601660000),
  ];
  clearAndDraw(data);
});
