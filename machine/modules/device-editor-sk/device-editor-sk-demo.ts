import { $$ } from 'common-sk/modules/dom';
import './index';
import { DeviceEditorSk } from './device-editor-sk';

if (window.location.hash === '#preexisting') {
  $$<DeviceEditorSk>('device-editor-sk')?.show({
    id: ['skia-machine-001'],
    gpu: ['some-gpu'],
    cpu: ['x86', 'x86_64'],
  }, 'root@skia-device-001');
} else if (window.location.hash === '#no_sshuserip') {
  $$<DeviceEditorSk>('device-editor-sk')?.show({
    id: ['skia-no-ssh-user-ip'],
    gpu: ['this-gpu-should-not-display'],
    cpu: ['x86', 'x86_64'],
  }, '');
} else {
  $$<DeviceEditorSk>('device-editor-sk')?.show({
    id: ['skia-machine-001'],
  }, '');
}
