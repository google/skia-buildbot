import './index.js'

import { $$ } from 'common-sk/modules/dom'

let ck = $$('canvaskit-fiddle');

ck.content = `const surface = CanvasKit.MakeCanvasSurface(canvas.id);
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

function drawFrame() {
  const path = starPath(CanvasKit, X, Y);
  CanvasKit.setCurrentContext(context);
  const dpe = CanvasKit.MakeSkDashPathEffect([15, 5, 5, 10], offset/5);
  offset++;

  paint.setPathEffect(dpe);
  paint.setStyle(CanvasKit.PaintStyle.Stroke);
  paint.setStrokeWidth(5.0 + -3 * Math.cos(offset/30));
  paint.setAntiAlias(true);
  paint.setColor(CanvasKit.Color(66, 129, 164, 1.0));

  skcanvas.clear(CanvasKit.Color(255, 255, 255, 1.0));

  skcanvas.drawPath(path, paint);
  skcanvas.flush();
  dpe.delete();
  path.delete();
  requestAnimationFrame(drawFrame);
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
});`
