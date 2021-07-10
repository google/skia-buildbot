import './machine-server-sk';
import fetchMock, { MockRequest, MockResponse } from 'fetch-mock';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import {
  MachineServerSk, MAX_LAST_UPDATED_ACCEPTABLE_MS, outOfSpecIfTooOld, pretty_device_name,
} from './machine-server-sk';
import { Annotation } from '../json';

fetchMock.config.overwriteRoutes = true;

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

const setUpElement = async (): Promise<MachineServerSk> => {
  fetchMock.reset();
  fetchMock.get('/_/machines', [
    {
      Mode: 'available',
      Battery: 100,
      PodName: 'rpi-swarming-123456-987',
      ScheduledForDeletion: '',
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
        LastUpdated: '2020-04-21T17:33:09.638275Z',
      },
      PowerCycle: false,
      LastUpdated: '2020-04-21T17:33:09.638275Z',
      Temperature: { dumpsys_battery: 26 },
    },
  ]);

  container.innerHTML = '<machine-server-sk></machine-server-sk>';
  const s = container.firstElementChild as MachineServerSk;

  // Wait for the initial fetch to finish.
  await fetchMock.flush(true);

  return s;
};

describe('machine-server-sk', () => {
  it('loads data by fetch on connectedCallback', async () => {
    const s = await setUpElement();

    // Each row has an id set to the machine id.
    assert.isNotNull($$('#skia-rpi2-rack4-shelf1-002', s));
  });

  it('filters out elements that do not match', async () => {
    const s = await setUpElement();

    assert.isNotNull($$('#skia-rpi2-rack4-shelf1-002', s));

    const filterElement = $$<HTMLInputElement>('#filter-input', s)!;
    filterElement.value = 'this string does not appear in any machine';
    filterElement.dispatchEvent(new InputEvent('input'));

    // Each row has an id set to the machine id.
    assert.isNull($$('#skia-rpi2-rack4-shelf1-002', s));
  });

  it('updates the mode when you click on the mode button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get('/_/machine/toggle_mode/skia-rpi2-rack4-shelf1-002', 200);
    fetchMock.get('/_/machines', [
      {
        Mode: 'maintenance',
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
          LastUpdated: '2020-04-21T17:33:09.638275Z',
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

  it('updates ScheduledForDeletion when you click on the update button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get(
      '/_/machine/toggle_update/skia-rpi2-rack4-shelf1-002',
      200,
    );
    fetchMock.get('/_/machines', [
      {
        Mode: 'maintenance',
        Battery: 100,
        PodName: 'rpi-swarming-123456-987',
        ScheduledForDeletion: 'rpi-swarming-123456-987',
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
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button.
    const button = $$<HTMLButtonElement>('button.update', s)!;
    button.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm the button text has been updated.
    assert.equal('Waiting for update.', button.textContent?.trim());
  }));

  it('starts requesting updates when you click on the refresh button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get('/_/machines', [
      {
        Mode: 'maintenance',
        Battery: 100,
        PodName: 'rpi-swarming-123456-987',
        ScheduledForDeletion: 'rpi-swarming-123456-987',
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
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button.
    $$<HTMLButtonElement>('#refresh', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm we are displaying the right icon.
    assert.isNotNull($$('pause-icon-sk', s));
  }));

  it('updates PowerCycle when you click on the button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get(
      '/_/machine/toggle_powercycle/skia-rpi2-rack4-shelf1-002',
      200,
    );
    fetchMock.get('/_/machines', [
      {
        Mode: 'maintenance',
        Battery: 100,
        PodName: 'rpi-swarming-123456-987',
        ScheduledForDeletion: 'rpi-swarming-123456-987',
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
          LastUpdated: '2020-04-21T17:33:09.638275Z',
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

    // Confirm the button text has been updated.
    assert.equal(
      'Waiting for Power Cycle',
      $$('.powercycle', s)?.textContent?.trim(),
    );
  }));

  it('clears the Dimensions when you click on the button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();
    // Confirm there are row in the dimensions.
    assert.isNotNull($$('details.dimensions table tr', s));

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get(
      '/_/machine/remove_device/skia-rpi2-rack4-shelf1-002',
      200,
    );
    fetchMock.get('/_/machines', [
      {
        Mode: 'maintenance',
        Battery: 100,
        PodName: 'rpi-swarming-123456-987',
        ScheduledForDeletion: 'rpi-swarming-123456-987',
        Dimensions: {},
        Note: {
          User: '',
          Message: '',
          Timestamp: '2020-04-21T17:33:09.638275Z',
        },
        Annotation: {
          User: '',
          Message: '',
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        PowerCycle: true,
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Click the button.
    $$<HTMLElement>('clear-icon-sk', s)!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);
  }));

  it('deletes the Machine when you click on the button', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const s = await setUpElement();

    // Confirm there are rows in the table.
    assert.isNotNull($$('main > table > tbody > tr > td', s));

    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    fetchMock.get(
      '/_/machine/delete_machine/skia-rpi2-rack4-shelf1-002',
      200,
    );
    fetchMock.get('/_/machines', []);

    // Click the button.
    $$<HTMLElement>('delete-icon-sk')!.click();

    // Wait for all requests to finish.
    await fetchMock.flush(true);

    // Confirm the one machine has been removed.
    assert.isNull($$('main > table > tbody > tr > td', s));
  }));

  it('sets the Machine Note when you edit the note.', () => window.customElements.whenDefined('machine-server-sk').then(async () => {
    const updatedMessage = 'This has been edited.';
    const s = await setUpElement();
    // Now set up fetchMock for the requests that happen when the button is clicked.
    fetchMock.reset();
    let called = false;
    fetchMock.post(
      '/_/machine/set_note/skia-rpi2-rack4-shelf1-002',
      (url: string, opts: MockRequest): MockResponse => {
        const body = JSON.parse(opts.body as string) as Annotation;
        assert.equal(body.Message, updatedMessage);
        called = true;
        return {};
      }, {
        sendAsJson: true,
      },
    );
    fetchMock.get('/_/machines', [
      {
        Mode: 'available',
        Battery: 100,
        PodName: 'rpi-swarming-123456-987',
        ScheduledForDeletion: '',
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
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        PowerCycle: false,
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    // Open the editor dialog.
    $$<HTMLElement>('edit-icon-sk', s)!.click();
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
