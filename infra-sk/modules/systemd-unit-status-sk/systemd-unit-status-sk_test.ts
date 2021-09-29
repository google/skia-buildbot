import './index';

import { expect } from 'chai';
import { setUpElementUnderTest } from '../test_util';
import { SystemdUnitStatusSk, SystemdUnitStatusSkEventDetail } from './systemd-unit-status-sk';

describe('systemd-unit-status-sk', () => {
  const newInstance = setUpElementUnderTest<SystemdUnitStatusSk>('systemd-unit-status-sk');

  describe('restart', () => {
    it('generates event when clicked', () => {
      const systemdUnitStatusSk = newInstance();
      systemdUnitStatusSk.setAttribute('machine', 'skia-fiddle');
      systemdUnitStatusSk.value = {
        status: {
          Name: 'pulld.service',
          Description: '',
          LoadState: '',
          ActiveState: '',
          SubState: '',
          Followed: '',
          Path: '',
          JobId: 0,
          JobType: '',
          JobPath: '',
        },
        props: {},
      };

      let detail: SystemdUnitStatusSkEventDetail;
      systemdUnitStatusSk.addEventListener('unit-action', (e) => {
        detail = (e as CustomEvent<SystemdUnitStatusSkEventDetail>).detail;
      });
      const button = systemdUnitStatusSk.querySelector<HTMLButtonElement>(
        'button[data-action=restart]',
      )!;
      expect(button.textContent).to.equal('Restart');
      button.click();
      expect(detail!.machine).to.equal('skia-fiddle');
      expect(detail!.name).to.equal('pulld.service');
      expect(detail!.action).to.equal('restart');
    });
  });
});
