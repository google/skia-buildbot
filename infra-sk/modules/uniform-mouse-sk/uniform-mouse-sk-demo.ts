import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { UniformMouseSk } from './uniform-mouse-sk';

const mouseUniformControl = $$<UniformMouseSk>('uniform-mouse-sk')!;
mouseUniformControl.elementToMonitor = $$<HTMLCanvasElement>('canvas')!;

const applyUniformValues = () => {
  const uniforms = [0, 0, 0, 0];
  $<UniformMouseSk>('uniform-mouse-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
};

$$<HTMLCanvasElement>('canvas')!.addEventListener('mousemove', applyUniformValues);
$$<HTMLCanvasElement>('canvas')!.addEventListener('mousedown', applyUniformValues);
$$<HTMLCanvasElement>('canvas')!.addEventListener('mouseup', applyUniformValues);
$$<HTMLCanvasElement>('canvas')!.addEventListener('click', applyUniformValues);
