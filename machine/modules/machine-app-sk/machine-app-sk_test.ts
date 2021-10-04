import './index';

import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { MachineAppSk } from './index';
import { FrontendDescription } from '../json';

/**
 * Mock the machine-server-sk JSON endpoint to return a valid response.
 */
function mockMachinesResponse(): void {
  const ARBITRARY_DATE = '2020-04-21T17:33:09.638275Z';
  const arbitraryResponse: Partial<FrontendDescription> = {
    Mode: 'available',
    Battery: 100,
    Dimensions: {
      id: ['skia-rpi2-rack4-shelf1-002'],
      android_devices: ['1'],
      device_os: ['H', 'HUAWEIELE-L29'],
    },
    Note: {
      User: '',
      Message: '',
      Timestamp: ARBITRARY_DATE,
    },
    Annotation: {
      User: '',
      Message: '',
      Timestamp: ARBITRARY_DATE,
    },
    PowerCycle: false,
    LastUpdated: ARBITRARY_DATE,
    Temperature: { dumpsys_battery: 26 },
  };
  fetchMock.get('/_/machines', arbitraryResponse);
}

const setUpElement = async (): Promise<MachineAppSk> => {
  fetchMock.reset();
  fetchMock.config.overwriteRoutes = true;
  mockMachinesResponse();

  document.body.innerHTML = '<machine-app-sk></machine-app-sk>';

  // Wait for the initial fetch to finish.
  await fetchMock.flush(true);

  return document.body.firstElementChild as MachineAppSk;
};

describe('machine-app-sk', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('starts requesting updates for the selected tab when you click on the refresh button',
    () => window.customElements.whenDefined('machine-app-sk').then(async () => {
      const s = await setUpElement();

      // Now set up fetchMock for the requests that happen when the button is clicked.
      fetchMock.reset();
      mockMachinesResponse();

      // Click the button.
      $$<HTMLButtonElement>('#refresh', s)!.click();

      // Wait for all requests to finish.
      await fetchMock.flush(true);

      // Confirm we are displaying the right icon.
      assert.isNotNull($$('pause-icon-sk', s));
    })
  );
});
