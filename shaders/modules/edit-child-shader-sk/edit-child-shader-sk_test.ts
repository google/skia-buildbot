import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { EditChildShaderSk } from './edit-child-shader-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ChildShader } from '../json';

const childShader: ChildShader = {
  UniformName: 'someUniformName',
  ScrapHashOrName: '@iExample',
};

describe('edit-child-shader-sk', () => {
  const newInstance = setUpElementUnderTest<EditChildShaderSk>('edit-child-shader-sk');

  let element: EditChildShaderSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('edit-child-shader-sk', () => {
    it('returns the childShader on OK.', async () => {
      const promise = element.show(childShader);
      $$<HTMLButtonElement>('#ok', element)!.click();
      assert.deepEqual(await promise, childShader);
    });

    it('returns undefined on Cancel.', async () => {
      const promise = element.show(childShader);
      $$<HTMLButtonElement>('#cancel', element)!.click();
      assert.isUndefined(await promise);
    });
  });
});
