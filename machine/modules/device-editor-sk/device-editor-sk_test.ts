import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { ClearDeviceEvent, DeviceEditorSk } from './device-editor-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('device-editor-sk', () => {
  const newInstance = setUpElementUnderTest<DeviceEditorSk>('device-editor-sk');

  let element: DeviceEditorSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('emits an event on deleting the device', async () => {
    const testID = 'the-test-machine-001';
    element.show(testID);
    const clearEvent = eventPromise<CustomEvent<string>>(ClearDeviceEvent, 100);
    $$<HTMLButtonElement>('button.clear', element)!.click();

    assert.equal((await clearEvent).detail, testID);
  });
});
