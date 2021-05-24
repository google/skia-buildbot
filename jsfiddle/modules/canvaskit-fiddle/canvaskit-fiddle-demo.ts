import './index';

import { $$ } from 'common-sk/modules/dom';
import { WasmFiddle } from '../wasm-fiddle/wasm-fiddle';

const ck = $$<WasmFiddle>('canvaskit-fiddle')!;

ck.content = `// One can specify up to 10 sliders or color pickers using the syntax
// #sliderN:displayNameNoSpaces. This will create a variable in the scope
// matching the left part (it's a normal HTML input tag) that has the part
// on the right as the display name in the html. #colorN is also valid for
// a native HTML color picker.
// #slider0:strokeWidth #color0:dashColor

const surface = CanvasKit.MakeCanvasSurface(canvas.id);
if (!surface) {
  throw 'Could not make surface';
}
const skcanvas = surface.getCanvas();
const paint = new CanvasKit.SkPaint();

let offset = 0;
let X = 250;
let Y = 250;

// If there are multiple contexts on the screen, we need to make sure
// we switch to this one before we draw.
const context = CanvasKit.currentContext();

// Set a default color
color0.value="#4746cd";

function getColor() {
  // color0.value is in #RRGGBB form
  // https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/color
  return CanvasKit._testing.parseColor(color0.value);
}

function getWidth() {
  // slider0.valueAsNumber is a float in the range [0, 1]
  return slider0.valueAsNumber * 10 + 3;
}

function drawFrame() {
  benchmarkFPS();
  const path = starPath(CanvasKit, X, Y);
  CanvasKit.setCurrentContext(context);
  const dpe = CanvasKit.SkPathEffect.MakeDash([15, 5, 5, 10], offset/5);
  offset++;

  paint.setPathEffect(dpe);
  paint.setStyle(CanvasKit.PaintStyle.Stroke);
  paint.setStrokeWidth(getWidth());
  paint.setAntiAlias(true);
  paint.setColor(getColor());

  skcanvas.clear(CanvasKit.Color(255, 255, 255, 1.0));

  skcanvas.drawPath(path, paint);
  skcanvas.flush();
  dpe.delete();
  path.delete();
  if (isRunning()) {
    requestAnimationFrame(drawFrame);
  }
}
requestAnimationFrame(drawFrame);

function starPath(CanvasKit, X, Y, R=128) {
  let p = new CanvasKit.SkPath();
  p.moveTo(X + R, Y);
  for (let i = 1; i < 8; i++) {
    let a = 2.6927937 * i;
    p.lineTo(X + R * Math.cos(a), Y + R * Math.sin(a));
  }
  return p;
}

// Make animation interactive
canvas.addEventListener('mousemove', (e) => {
  X = e.offsetX;
  Y = e.offsetY;
});`;
