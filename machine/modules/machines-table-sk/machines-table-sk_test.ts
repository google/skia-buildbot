import './machines-table-sk';
import fetchMock, { MockRequest, MockResponse } from 'fetch-mock';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import {
  Mode, AttachedDevice, Annotation, SwarmingDimensions,
} from '../json';
import {
  MachinesTableSk, MAX_LAST_UPDATED_ACCEPTABLE_MS, outOfSpecIfTooOld, pretty_device_name_as_string, sortByAnnotation, sortByAttachedDevice, sortByBattery, sortByDevice, sortByDeviceUptime, sortByLastUpated, sortByLaunchedSwarming, sortByMachineID, sortByMode, sortByNote, sortByPowerCycle, sortByQuarantined, sortByRunningSwarmingTask, sortByVersion,
} from './machines-table-sk';
import {
  FrontendDescription, ListMachinesResponse, SetNoteRequest,
} from '../json';
import { compareFunc } from '../sort';

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

  it('clears the Dimensions when you click on the button in Dimensions', () => window.customElements.whenDefined('machines-table-sk').then(async () => {
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
    $$<HTMLElement>('clear-all-icon-sk', s)!.click();

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
    assert.isNotNull($$('table > tbody > tr > td.powercycle', s));

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
    assert.isNull($$('table > tbody > tr > td.powercycle', s));
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
      const machine: Partial<FrontendDescription> = { LastUpdated: now.toString() };
      assert.equal(outOfSpecIfTooOld(machine as FrontendDescription), '');
    });

    it('returns outOfSpec if LastModified is too old', () => {
      const old = new Date(Date.now() - 2 * MAX_LAST_UPDATED_ACCEPTABLE_MS);
      const machine: Partial<FrontendDescription> = { LastUpdated: old.toString() };
      assert.equal(outOfSpecIfTooOld(machine as FrontendDescription), 'outOfSpec');
    });
  });

  describe('pretty_device_name', () => {
    it('returns an empty string on null', () => {
      const machine: Partial<FrontendDescription> = { Dimensions: { } };
      assert.equal('', pretty_device_name_as_string(machine as FrontendDescription));
    });
    it('returns Pixel 5 for redfin.', () => {
      const machine: Partial<FrontendDescription> = { Dimensions: { device_type: ['redfin'] } };
      assert.equal('redfin (Pixel 5)', pretty_device_name_as_string(machine as FrontendDescription));
    });
    it('returns the last match in a list', () => {
      const machine: Partial<FrontendDescription> = { Dimensions: { device_type: ['herolte', 'universal8890'] } };
      assert.equal('herolte | universal8890 (Galaxy S7 [Global])', pretty_device_name_as_string(machine as FrontendDescription));
    });
  });

  // Utiltiy function that tests the compare function passed in against
  // FrontendDescriptions with the value of its 'key' set to 'aValue' and
  // 'bValue' respectively. Note that the values passed in must be in the order
  // aValue < bValue.
  const testCompareFunc = <T>(key: string, fn: compareFunc<FrontendDescription>, aValue: T, bValue: T) => {
    const a: Record<string, T> = {};
    a[key] = aValue;
    const b: Record<string, T> = {};
    b[key] = bValue;

    const castFn = fn as unknown as compareFunc<Record<string, T>>;
    assert.isBelow(castFn(a, b), 0, key);
    assert.isAbove(castFn(b, a), 0, key);
    assert.equal(castFn(b, b), 0, key);
    assert.equal(castFn(a, a), 0, key);
  };

  describe('compare functions', () => {
    it('returns correct values on simple compares', () => {
      testCompareFunc<Mode>('Mode', sortByMode, 'available', 'maintenance');
      testCompareFunc<AttachedDevice>('AttachedDevice', sortByAttachedDevice, 'adb', 'nodevice');
      testCompareFunc<Annotation>('Annotation', sortByAnnotation, { Message: 'a' } as Annotation, { Message: 'b' } as Annotation);
      testCompareFunc<Annotation>('Note', sortByNote, { Message: 'a' } as Annotation, { Message: 'b' } as Annotation);
      testCompareFunc<string>('Version', sortByVersion, 'v001', 'v002');
      testCompareFunc<string>('LastUpdated', sortByLastUpated, '2022-03-03T22:22:22.222222Z', '2022-03-03T44:44:44.444444Z');
      testCompareFunc<number>('Battery', sortByBattery, 50, 100);
      testCompareFunc<boolean>('RunningSwarmingTask', sortByRunningSwarmingTask, false, true);
      testCompareFunc<boolean>('LaunchedSwarming', sortByLaunchedSwarming, false, true);
      testCompareFunc<number>('DeviceUptime', sortByDeviceUptime, 10, 20);
      testCompareFunc<SwarmingDimensions>('Dimensions', sortByDevice, { device_type: ['a'] }, { device_type: ['b'] });
      testCompareFunc<SwarmingDimensions>('Dimensions', sortByQuarantined, { quarantined: ['a'] }, { quarantined: ['b'] });
      testCompareFunc<SwarmingDimensions>('Dimensions', sortByMachineID, { id: ['a'] }, { id: ['b'] });
    });

    it('sortByPowerCycle', () => {
      const a: Record<string, any> = {};
      a.PowerCycle = true;
      a.PowerCycleState = 'available';
      const b: Record<string, any> = {};
      b.PowerCycle = true;
      b.PowerCycleState = 'not_available';

      const castFn = sortByPowerCycle as unknown as compareFunc<Record<string, any>>;
      assert.isBelow(castFn(a, b), 0, 'sortByPowerCycle');
      assert.isAbove(castFn(b, a), 0, 'sortByPowerCycle');
      assert.equal(castFn(b, b), 0, 'sortByPowerCycle');
      assert.equal(castFn(a, a), 0, 'sortByPowerCycle');
    });
  });
});
