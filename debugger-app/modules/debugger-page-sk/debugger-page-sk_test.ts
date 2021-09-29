import './index';
import { expect } from 'chai';
import { DebuggerPageSk } from './debugger-page-sk';
import { testData } from '../commands-sk/test-data';
import { CommandsSk } from '../commands-sk/commands-sk';
import { HistogramSk } from '../histogram-sk/histogram-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

function hasClass(e: HTMLElement, expected: string): boolean {
  const c = e.getAttribute('class');
  if (!c) { return false; }
  return e.classList.contains(expected);
}

// debugger-page-sk expects to find this defined
const SKIA_VERSION = 'test';

describe('debugger-page-sk', () => {
  const newInstance = setUpElementUnderTest<DebuggerPageSk>('debugger-page-sk');

  let debuggerPageSk: DebuggerPageSk;
  beforeEach(() => {
    debuggerPageSk = newInstance((el: DebuggerPageSk) => {});
  });

  // In addition to showing counts, the histogram functions as a way to toggle any
  // command in or out of the filter. is is integrated closely with commandsSk, and
  // that integration is tested here.
  describe('commands-histogram interaction', () => {
    it('can set a negative text filter and it is reflected in the histogram', () => {
      const commandsSk = debuggerPageSk.querySelector<CommandsSk>('commands-sk')!;
      const histogramSk = debuggerPageSk.querySelector<HistogramSk>('histogram-sk')!;

      commandsSk.clearFilter();
      commandsSk.processCommands(testData);
      commandsSk.textFilter = '!Restore Save';

      // expect histogram selection are highlighted correctly
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-save',
      )!, 'pinkBackground')).to.equal(true);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-restore',
      )!, 'pinkBackground')).to.equal(true);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-drawannotation',
      )!, 'pinkBackground')).to.equal(false);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-drawimagerect',
      )!, 'pinkBackground')).to.equal(false);
    });

    it('can set a postive text filter and it is reflected in the histogram', () => {
      const commandsSk = debuggerPageSk.querySelector<CommandsSk>('commands-sk')!;
      const histogramSk = debuggerPageSk.querySelector<HistogramSk>('histogram-sk')!;

      commandsSk.clearFilter();
      commandsSk.processCommands(testData);
      commandsSk.textFilter = 'Restore Save';

      // expect histogram selection are highlighted correctly
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-save',
      )!, 'pinkBackground')).to.equal(false);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-restore',
      )!, 'pinkBackground')).to.equal(false);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-drawannotation',
      )!, 'pinkBackground')).to.equal(true);
      expect(hasClass(histogramSk.querySelector<HTMLElement>(
        '#hist-row-drawimagerect',
      )!, 'pinkBackground')).to.equal(true);
    });

    it('can click a histogram row and reflect it in the text filter', () => {
      const commandsSk = debuggerPageSk.querySelector<CommandsSk>('commands-sk')!;
      const histogramSk = debuggerPageSk.querySelector<HistogramSk>('histogram-sk')!;

      commandsSk.clearFilter();
      commandsSk.processCommands(testData);

      // expect histogram selection are highlighted correctly
      histogramSk.querySelector<HTMLElement>('#hist-row-save')!.click();

      expect(commandsSk.querySelector<HTMLInputElement>(
        '#text-filter',
      )!.value).to.equal('!save');
    });
  });
});
