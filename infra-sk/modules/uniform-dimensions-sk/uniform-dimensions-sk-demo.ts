import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { DimensionsChangedEventDetail, dimensionsChangedEventName, UniformDimensionsSk } from './uniform-dimensions-sk';

$$('#apply')!.addEventListener('click', () => {
  // Pick a larger than needed uniforms size to show we don't affect the other uniform values.
  const uniforms = [0, 0, 0, 0, 0];
  $<UniformDimensionsSk>('uniform-dimensions-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});

$$('uniform-dimensions-sk')!.addEventListener(dimensionsChangedEventName, (e: Event) => {
  $$<HTMLPreElement>('#results')!.innerText = JSON.stringify((e as CustomEvent<DimensionsChangedEventDetail>).detail);
});
