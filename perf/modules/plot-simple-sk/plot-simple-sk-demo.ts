import './index';
import { $, $$ } from 'common-sk/modules/dom';
import {
  PlotSimpleSk,
  PlotSimpleSkTraceEventDetails,
  PlotSimpleSkZoomEventDetails,
} from './plot-simple-sk';
import 'elements-sk/styles/buttons';

import '../../../infra-sk/modules/theme-chooser-sk';

// Create our own random number generator that's deterministic so that we get
// consistent Gold images.
let seed = 1;
const MAX = 1e20;
const random = (): number => {
  seed = (seed * 999331) /* a prime number */ % MAX;
  return seed / MAX;
};

window.customElements.whenDefined('plot-simple-sk').then(() => {
  const ele = $$<PlotSimpleSk>('#plot')!;
  let n = 0;

  function add(plot: PlotSimpleSk, num: number) {
    const labels = [];
    for (let i = 0; i < 50; i++) {
      labels.push(new Date(1554143900000 + i * i * 5 * 1000 * 60));
    }

    const traces: { [name: string]: number[] } = {};
    for (let j = 0; j < num; j++) {
      const trace = [];
      for (let i = 0; i < 50; i++) {
        if (random() < 0.9) {
          trace.push(1000000 * (8 + Math.sin(i / 10) + j + random() * 1 + 10));
        } else {
          trace.push(1e32);
        }
      }
      const id = `trace${j + n}`;
      traces[id] = trace;
    }
    n += num;
    plot.addLines(traces, labels);
  }

  $<PlotSimpleSk>('plot-simple-sk').forEach((plot) => {
    add(plot, 10);
  });

  $$<HTMLButtonElement>('#add')!.addEventListener('click', () => {
    add(ele, 10);
  });

  $$<HTMLButtonElement>('#addalot')!.addEventListener('click', () => {
    add(ele, 100);
  });

  $$<HTMLButtonElement>('#clear')!.addEventListener('click', () => {
    ele.removeAll();
  });

  $$<HTMLButtonElement>('#reset')!.addEventListener('click', () => {
    ele.zoom = null;
  });

  $$<HTMLButtonElement>('#high')!.addEventListener('click', () => {
    ele.highlight = ['trace0', 'trace1'];
  });

  $$<HTMLButtonElement>('#clearhigh')!.addEventListener('click', () => {
    ele.highlight = [];
  });

  $$<HTMLButtonElement>('#xbar')!.addEventListener('click', () => {
    ele.xbar = 3;
  });

  $$<HTMLButtonElement>('#clearxbar')!.addEventListener('click', () => {
    ele.xbar = -1;
  });

  $$<HTMLButtonElement>('#zoomAction')!.addEventListener('click', () => {
    ele.zoom = [20, 40];
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_selected', (e) => {
    $$('#selected')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail,
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_focused', (e) => {
    $$('#focused')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail,
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('zoom', (e) => {
    $$('#zoom')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkZoomEventDetails>).detail,
    );
  });

  $$<HTMLButtonElement>('#bands')!.addEventListener('click', () => {
    ele.bands = [1, 4, 20, 30];
  });

  $$<HTMLButtonElement>('#toggleDots')!.addEventListener('click', () => {
    ele.dots = !ele.dots;
  });

  $$<HTMLButtonElement>('#special')!.addEventListener('click', () => {
    const trace = [];
    for (let i = 0; i < 50; i++) {
      trace.push(0);
    }
    ele.addLines({ specialZero: trace }, []);
  });
});
