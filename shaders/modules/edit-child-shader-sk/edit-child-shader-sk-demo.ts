import { $$ } from 'common-sk/modules/dom';
import { EditChildShaderSk } from './edit-child-shader-sk';
import './index';

$$('#editAction')!.addEventListener('click', async () => {
  const resp = await $$<EditChildShaderSk>('edit-child-shader-sk')!.show({
    UniformName: 'startingName',
    ScrapHashOrName: '@iExample',
  });
  $$('#results')!.textContent = JSON.stringify(resp, null, '  ');
});

$$<EditChildShaderSk>('edit-child-shader-sk')!.show({
  UniformName: 'startingName',
  ScrapHashOrName: '@iExample',
});
