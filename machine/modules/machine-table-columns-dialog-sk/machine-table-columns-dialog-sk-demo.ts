import { $$ } from 'common-sk/modules/dom';
import './index';
import { MachineTableColumnsDialogSk } from './machine-table-columns-dialog-sk';

$$('#open')!.addEventListener('click', async () => {
  $$<HTMLPreElement>('#results')!.textContent = JSON.stringify(
    await $$<MachineTableColumnsDialogSk>('machine-table-columns-dialog-sk')!.edit(['Launched Swarming', 'Note', 'Annotation']),
    null, '  ',
  );
});
