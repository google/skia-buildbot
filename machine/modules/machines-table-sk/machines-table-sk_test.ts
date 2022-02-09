import './machines-table-sk';
import fetchMock, { MockRequest, MockResponse } from 'fetch-mock';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import {
  MachinesTableSk, MAX_LAST_UPDATED_ACCEPTABLE_MS, outOfSpecIfTooOld, pretty_device_name,
} from './machines-table-sk';
import {
  FrontendDescription, ListMachinesResponse, SetNoteRequest,
} from '../json';

function mockMachinesResponse(param: ListMachinesResponse | Partial<FrontendDescription>[]): void {
  fetchMock.get('/_/machines', param);
}

const setUpElement = async (): Promise<MachinesTableSk> => {
  fetchMock.reset();
  fetchMock.config.overwriteRoutes = true;
  mockMachinesResponse([
    {
      Mode: 'available',
      AttachedDevice: 'ssh',
      Battery: 100,
      Dimensions: {
        id: ['skia-rpi2-rack4-shelf1-002'],
        android_devices: ['1'],
        device_os: ['H', 'HUAWEIELE-L29'],
      },
      Note: {
        User: '',
        Message: 'Starting note.',
        Timestamp: '2020-04-21T17:33:09.638275Z',
      },
      Annotation: {
        User: '',
        Message: '',
        Timestamp: '2020-04-21T17:33:09.638275Z',
      },
      PowerCycle: false,
      LastUpdated: '2020-04-21T17:33:09.638275Z',
      Temperature: { dumpsys_battery: 26 },
    },
  ]);

  document.body.innerHTML = '<machines-table-sk></machines-table-sk>';
  const element = document.body.firstElementChild as MachinesTableSk;
  await element.update();

  // Wait for the initial fetch to finish.
  await fetchMock.flush(true);

  return element;
};

describe('machines-table-sk', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('updates the mode when you click on the mode button', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.post('/_/machine/toggle_mode/skia-rpi2-rack4-shelf1-002', 200);
    mockMachinesResponse([
      {
        Mode: 'maintenance',
        AttachedDevice: 'ssh',
        Battery: 100,
        Dimensions: {
          id: ['skia-rpi2-rack4-shelf1-002'],
          android_devices: ['1'],
          device_os: ['H', 'HUAWEIELE-L29'],
        },
        Note: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        Annotation: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button.
    const button = $$<HTMLButtonElement>('button.mode', s)!;
    button.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm the button text has been updated.
    assert.equal('maintenance', button.textContent?.trim());
  }));

  it('updates PowerCycle when you click on the button', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.post(
      '/_/machine/toggle_powercycle/skia-rpi2-rack4-shelf1-002',
      200,
    );
    mockMachinesResponse([
      {
        Mode: 'maintenance',
        AttachedDevice: 'ssh',
        Battery: 100,
        Dimensions: {
          id: ['skia-rpi2-rack4-shelf1-002'],
          android_devices: ['1'],
          device_os: ['H', 'HUAWEIELE-L29'],
        },
        Note: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        Annotation: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        PowerCycle: true,
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button.
    $$<HTMLElement>('power-settings-new-icon-sk', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm the spinner is active.
    assert.isTrue(
      $$<SpinnerSk>('.powercycle spinner-sk', s)!.active,
    );
  }));

  it('clears the Dimensions when you click on the button', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const s = await setUpElement();
    // Confirm there are row in the dimensions.
    assert.isNotNull($$('details.dimensions table tr', s));

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    let called = false;
    fetchMock.post(
      (url: string): boolean => {
        if (url !== '/_/machine/remove_device/skia-rpi2-rack4-shelf1-002') {
          return false;
        }
        called = true;
        return true;
      }, 200,
    );
    mockMachinesResponse([
      {
        Mode: 'maintenance',
        AttachedDevice: 'ssh',
        Battery: 100,
        Dimensions: {},
        Note: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        Annotation: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        PowerCycle: true,
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button to show the dialog
    $$<HTMLElement>('edit-icon-sk.edit_device', s)!.click();
    // Now clear the dimensions
    $$<HTMLElement>('device-editor-sk button.clear', s)!.click();
    $$<HTMLElement>('device-editor-sk button.clear_yes_im_sure', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);
    assert.isTrue(called);
  }));

  it('supplies chrome os data via RPC', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const s = await setUpElement();
    // Confirm there are row in the dimensions.
    assert.isNotNull($$('details.dimensions table tr', s));

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    let called = false;
    fetchMock.post(
      (url: string, opts: MockRequest): boolean => {
        if (url !== '/_/machine/supply_chromeos/skia-rpi2-rack4-shelf1-002') {
          return false;
        }
        assert.equal(opts.body, '{"SSHUserIP":"root@test-chrome-os","SuppliedDimensions":{"gpu":["Mali999"],"cpu":["arm","arm64"]}}');
        called = true;
        return true;
      },
      200,
    );
    mockMachinesResponse([]);

    // Click the button to show the dialog
    $$<HTMLElement>('edit-icon-sk.edit_device', s)!.click();
    $$<HTMLInputElement>('device-editor-sk input#user_ip', s)!.value = 'root@test-chrome-os';
    $$<HTMLInputElement>('device-editor-sk input#chromeos_gpu', s)!.value = 'Mali999';
    $$<HTMLInputElement>('device-editor-sk input#chromeos_cpu', s)!.value = 'arm,arm64';
    // Now apply those dimensions
    $$<HTMLElement>('device-editor-sk button.apply', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);
    assert.isTrue(called);
  }));

  it('deletes the Machine when you click on the button', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const s = await setUpElement();

    // Confirm there are rows in the table.
    assert.isNotNull($$('table > tbody > tr[id] > td', s));

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.post(
      '/_/machine/delete_machine/skia-rpi2-rack4-shelf1-002',
      200,
    );
    mockMachinesResponse([]);

    // Click the button.
    $$<HTMLElement>('delete-icon-sk')!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm the one machine has been removed.
    assert.isNull($$('table > tbody > tr[id] > td', s));
  }));

  it('sets the Machine Note when you edit the note.', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
    const updatedMessage = 'This has been edited.';
    const s = await setUpElement();
    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    let called = false;
    fetchMock.post(
      '/_/machine/set_note/skia-rpi2-rack4-shelf1-002',
      (url: string, opts: MockRequest): MockResponse => {
        const body = JSON.parse(opts.body as string) as SetNoteRequest;
        assert.equal(body.Message, updatedMessage);
        called = true;
        return {};
      }, {
        sendAsJson: true,
      },
    );
    mockMachinesResponse([
      {
        Mode: 'available',
        AttachedDevice: 'ssh',
        Battery: 100,
        Dimensions: {
          id: ['skia-rpi2-rack4-shelf1-002'],
          android_devices: ['1'],
          device_os: ['H', 'HUAWEIELE-L29'],
        },
        Note: {
          User: '',
          Message: updatedMessage,
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        Annotation: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        PowerCycle: false,
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Open the editor dialog.
    $$<HTMLElement>('edit-icon-sk.edit_note', s)!.click();
    // Change the message.
    $$<HTMLInputElement>('note-editor-sk #note', s)!.value = updatedMessage;
    // Press OK.
    $$<HTMLInputElement>('note-editor-sk #ok', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    assert.isTrue(called);
  }));

  describe('outOfSpecIfTooOld', () => {
    it('returns an empty string if LastModified is recent enough', () => {
      const now = new Date(Date.now());
      assert.equal(outOfSpecIfTooOld(now.toString()), '');
    });

    it('returns outOfSpec if LastModified is too old', () => {
      const old = new Date(Date.now() - 2 * MAX_LAST_UPDATED_ACCEPTABLE_MS);
      assert.equal(outOfSpecIfTooOld(old.toString()), 'outOfSpec');
    });
  });

  describe('pretty_device_name', () => {
    it('returns an empty string on null', () => {
      assert.equal('', pretty_device_name(null));
    });
    it('returns Pixel 5 for redfin.', () => {
      assert.equal('redfin (Pixel 5)', pretty_device_name(['redfin']));
    });
    it('returns the last match in a list', () => {
      assert.equal('herolte | universal8890 (Galaxy S7 [Global])', pretty_device_name(['herolte', 'universal8890']));
    });
  });
});
