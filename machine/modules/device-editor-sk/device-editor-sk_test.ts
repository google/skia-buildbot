import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import {
  ClearDeviceEvent, DeviceEditorSk, UpdateDimensionsDetails, UpdateDimensionsEvent,
} from './device-editor-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('device-editor-sk', () => {
  const newInstance = setUpElementUnderTest<DeviceEditorSk>('device-editor-sk');

  let element: DeviceEditorSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('emits an event on deleting the device', async () => {
    const testID = 'the-test-machine-001';
    element.show({
      id: [testID],
    }, '');
    const clearEvent = eventPromise<CustomEvent<string>>(ClearDeviceEvent, 100);
    $$<HTMLButtonElement>('.info button.clear', element)!.click();

    $$<HTMLButtonElement>('.confirm button.clear_yes_im_sure', element)!.click();

    assert.equal((await clearEvent).detail, testID);
  });

  it('emits an event with updated values after hitting apply', async () => {
    const testID = 'the-test-machine-001';
    element.show({
      id: [testID],
    }, '');

    $$<HTMLInputElement>('input#user_ip', element)!.value = 'root@new-device';
    $$<HTMLInputElement>('input#chromeos_gpu', element)!.value = 'my-gpu,my-other-gpu';
    $$<HTMLInputElement>('input#chromeos_cpu', element)!.value = 'arm64,arm';

    const updateEvent = eventPromise<CustomEvent<UpdateDimensionsDetails>>(UpdateDimensionsEvent, 100);
    $$<HTMLButtonElement>('button.apply', element)!.click();

    assert.deepEqual((await updateEvent).detail, {
      machineID: testID,
      sshUserIP: 'root@new-device',
      specifiedDimensions: {
        gpu: ['my-gpu', 'my-other-gpu'],
        cpu: ['arm64', 'arm'],
      },
    });
  });
});
