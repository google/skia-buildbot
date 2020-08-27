import './machine-server-sk';
import { MachineServerSk } from './machine-server-sk';
import fetchMock from 'fetch-mock';
import { assert } from 'chai';

fetchMock.config.overwriteRoutes = true;

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('machine-server-sk', () => {
  describe('loads data by fetch on connectedCallback', () => {
    fetchMock.get('/_/machines', [
      {
        Mode: 'available',
        Battery: 100,
        Dimensions: {
          id: ['skia-rpi2-rack4-shelf1-002'],
          android_devices: ['1'],
          device_os: ['H', 'HUAWEIELE-L29'],
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

    it('fetches', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
        container.innerHTML = '<machine-server-sk></machine-server-sk>';
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);
        // Each row has an id set to the machine id.
        assert.isNotNull(s!.querySelector('#skia-rpi2-rack4-shelf1-002'));
      }));
  });

  describe('toggles maintenance mode on click', () => {
    fetchMock.get('/_/machines', [
      {
        Mode: 'available',
        Battery: 100,
        Dimensions: {
          id: ['skia-rpi2-rack4-shelf1-002'],
          android_devices: ['1'],
          device_os: ['H', 'HUAWEIELE-L29'],
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

    it('updates the mode when you click on the mode button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
        container.innerHTML = '<machine-server-sk></machine-server-sk>';
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

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
        const button = s!.querySelector<HTMLButtonElement>('button.mode')!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm the button text has been updated.
        assert.equal('maintenance', button.textContent?.trim());
      }));
  });

  describe('toggles ScheduledForDeletion on click', () => {
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
        Annotation: {
          User: '',
          Message: '',
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    it('updates ScheduledForDeletion when you click on the update button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
        container.innerHTML = '<machine-server-sk></machine-server-sk>';
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

        // Now set up fetchMock for the requests that happen when the button is clicked.
        fetchMock.reset();
        fetchMock.get(
          '/_/machine/toggle_update/skia-rpi2-rack4-shelf1-002',
          200
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
        const button = s!.querySelector<HTMLButtonElement>('button.update')!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm the button text has been updated.
        assert.equal('Waiting for update.', button.textContent?.trim());
      }));
  });

  describe('toggles refresh mode on click', () => {
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
        Annotation: {
          User: '',
          Message: '',
          LastUpdated: '2020-04-21T17:33:09.638275Z',
        },
        LastUpdated: '2020-04-21T17:33:09.638275Z',
        Temperature: { dumpsys_battery: 26 },
      },
    ]);

    it('starts requesting updates when you click on the refresh button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
        container.innerHTML = '<machine-server-sk></machine-server-sk>';
        const s = container.firstElementChild as MachineServerSk;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

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
        const button = s!.querySelector<HTMLButtonElement>('#refresh')!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm that setTimeout is in progress.
        assert.notEqual(0, s._timeout);
        // Confirm we are displaying the right icon.
        assert.isNotNull(s!.querySelector('pause-icon-sk'));
      }));
  });

  describe('toggles PowerCycle on click', () => {
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

    it('updates PowerCycle when you click on the button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
        container.innerHTML = '<machine-server-sk></machine-server-sk>';
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

        // Now set up fetchMock for the requests that happen when the button is clicked.
        fetchMock.reset();
        fetchMock.get(
          '/_/machine/toggle_powercycle/skia-rpi2-rack4-shelf1-002',
          200
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
        const button = s!.querySelector<HTMLElement>(
          'power-settings-new-icon-sk'
        )!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm the button text has been updated.
        assert.equal(
          'Waiting for Power Cycle',
          s!.querySelector('.powercycle')?.textContent?.trim()
        );
      }));
  });

  describe('clears Dimensions on click', () => {
    it('clears the Dimensions when you click on the button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
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
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

        // Confirm there are row in the dimensions.
        assert.isNotNull(s!.querySelector('details.dimensions table tr'));

        // Now set up fetchMock for the requests that happen when the button is clicked.
        fetchMock.reset();
        fetchMock.get(
          '/_/machine/remove_device/skia-rpi2-rack4-shelf1-002',
          200
        );
        fetchMock.get('/_/machines', [
          {
            Mode: 'maintenance',
            Battery: 100,
            PodName: 'rpi-swarming-123456-987',
            ScheduledForDeletion: 'rpi-swarming-123456-987',
            Dimensions: {},
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
        const button = s!.querySelector<HTMLElement>('clear-icon-sk')!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm the dimensions are now empty.
        assert.isNull(s!.querySelector('details.dimensions table tr'));
      }));
  });

  describe('deletes Machine on click', () => {
    it('deletes the Machine when you click on the button', () =>
      window.customElements.whenDefined('machine-server-sk').then(async () => {
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
        const s = container.firstElementChild;

        // Wait for the initial fetch to finish.
        await fetchMock.flush(true);

        // Confirm there are rows in the table.
        assert.isNotNull(s!.querySelector('main > table > tbody > tr > td'));

        // Now set up fetchMock for the requests that happen when the button is clicked.
        fetchMock.reset();
        fetchMock.get(
          '/_/machine/delete_machine/skia-rpi2-rack4-shelf1-002',
          200
        );
        fetchMock.get('/_/machines', []);

        // Click the button.
        const button = s!.querySelector<HTMLElement>('delete-icon-sk')!;
        button.click();

        // Wait for all requests to finish.
        await fetchMock.flush(true);

        // Confirm the one machine has been removed.
        assert.isNull(s!.querySelector('main > table > tbody > tr > td'));
      }));
  });
});
