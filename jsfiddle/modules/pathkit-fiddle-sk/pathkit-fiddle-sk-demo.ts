import './index';

import { $$ } from 'common-sk/modules/dom';
import { WasmFiddle } from '../wasm-fiddle-sk/wasm-fiddle-sk';

const pk = $$ <WasmFiddle>('pathkit-fiddle')!;
pk.content = `// canvas and PathKit are globally available
let firstPath = PathKit.FromSVGString('M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z');

let secondPath = PathKit.NewPath();
// Acts somewhat like the Canvas API, except can be chained
secondPath.moveTo(1, 1)
          .lineTo(20, 1)
          .lineTo(10, 30)
          .closePath();

// Join the two paths together (mutating firstPath in the process)
firstPath.op(secondPath, PathKit.PathOp.INTERSECT);

// Draw directly to Canvas
let ctx = canvas.getContext('2d');
ctx.strokeStyle = '#CC0000';
ctx.fillStyle = '#000000';
ctx.scale(20, 20);
ctx.beginPath();
firstPath.toCanvas(ctx);
ctx.fill();
ctx.stroke();


// clean up WASM memory
// See http://kripken.github.io/emscripten-site/docs/porting/connecting_cpp_and_javascript/embind.html?highlight=memory#memory-management
firstPath.delete();
secondPath.delete();`;
