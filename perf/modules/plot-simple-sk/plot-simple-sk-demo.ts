import './index';
import {
  PlotSimpleSk,
  PlotSimpleSkTraceEventDetails,
  PlotSimpleSkZoomEventDetails,
} from './plot-simple-sk';
import 'elements-sk/styles/buttons';
import { $, $$ } from 'common-sk/modules/dom';
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
  var ele = $$<PlotSimpleSk>('#plot')!;
  var n = 0;

  function add(ele: PlotSimpleSk, num: number) {
    let labels = [];
    for (let i = 0; i < 50; i++) {
      labels.push(new Date(1554143900000 + i * i * 5 * 1000 * 60));
    }

    var traces: { [name: string]: number[] } = {};
    for (var j = 0; j < num; j++) {
      var trace = [];
      for (var i = 0; i < 50; i++) {
        if (random() < 0.9) {
          trace.push(1000000 * (8 + Math.sin(i / 10) + j + random() * 1 + 10));
        } else {
          trace.push(1e32);
        }
      }
      var id = 'trace' + (j + n);
      traces[id] = trace;
    }
    n += num;
    ele.addLines(traces, labels);
  }

  $<PlotSimpleSk>('plot-simple-sk').forEach((ele) => {
    add(ele, 10);
  });

  $$<HTMLButtonElement>('#add')!.addEventListener('click', function () {
    add(ele, 10);
  });

  $$<HTMLButtonElement>('#addalot')!.addEventListener('click', function () {
    add(ele, 100);
  });

  $$<HTMLButtonElement>('#clear')!.addEventListener('click', function () {
    ele.removeAll();
  });

  $$<HTMLButtonElement>('#reset')!.addEventListener('click', function () {
    ele.zoom = null;
  });

  $$<HTMLButtonElement>('#high')!.addEventListener('click', function (e) {
    ele.highlight = ['trace' + (n - 1), 'trace' + (n - 2)];
  });

  $$<HTMLButtonElement>('#clearhigh')!.addEventListener('click', function (e) {
    ele.highlight = [];
  });

  $$<HTMLButtonElement>('#xbar')!.addEventListener('click', function (e) {
    ele.xbar = 3;
  });

  $$<HTMLButtonElement>('#clearxbar')!.addEventListener('click', function (e) {
    ele.xbar = -1;
  });

  $$<HTMLButtonElement>('#zoomAction')!.addEventListener('click', function (e) {
    ele.zoom = [20, 40];
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_selected', function (e) {
    $$('#selected')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_focused', function (e) {
    $$('#focused')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('zoom', function (e) {
    $$('#zoom')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkZoomEventDetails>).detail
    );
  });

  $$<HTMLButtonElement>('#bands')!.addEventListener('click', function (e) {
    ele.bands = [1, 4, 20, 30];
  });

  $$<HTMLButtonElement>('#special')!.addEventListener('click', function (e) {
    var trace = [];
    for (var i = 0; i < 50; i++) {
      trace.push(0);
    }
    ele.addLines({ specialZero: trace }, []);
  });
});
