import './index';
import { assert } from 'chai';
import { PlotSummarySk } from './plot-summary-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { generateFullDataFrame } from '../dataframe/test_utils';
import { LitElement } from 'lit';

describe('plot-summary-sk', () => {
  const now = new Date('2024/9/20').getTime();
  const timeSpans = [6 * 60 * 60]; // 6 hours
  const commitRange = { begin: 100, end: 110 };
  const df = generateFullDataFrame(commitRange, now, 1, timeSpans);
  const newEl = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  const chartReady = (cb: () => LitElement) =>
    new Promise<LitElement>((resolve) => {
      const el = cb();
      el.addEventListener('google-chart-ready', () => {
        resolve(el);
      });
    }).then((el) => {
      return el.updateComplete;
    });

  ['canvas', 'material'].forEach((mode) =>
    describe(`selection in mode ${mode}`, () => {
      it('select an area', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
          el.selectionType = mode as any;
        });
        await element.updateComplete;
        await chartReady(() => {
          element.dataframe = df;
          return element;
        });

        const header = df.header!,
          start = 2,
          end = 6;
        element.Select(header![start]!, header![end]!);

        assert.approximately(element.selectedValueRange!.begin, header[start]!.offset, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.offset, 1e-3);
      });

      it('select an area before the chart is ready', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
          el.selectionType = mode as any;
        });
        await element.updateComplete;

        const header = df.header!,
          start = 2,
          end = 6;
        element.dataframe = df;
        element.Select(header![start]!, header![end]!);

        assert.isNull(element['chartLayout']);
        assert.isNull(element['selectionRange']);
        assert.approximately(element.selectedValueRange!.begin!, 0, 1e-3);

        await chartReady(() => {
          return element;
        });

        assert.approximately(element.selectedValueRange!.begin, header[start]!.offset, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.offset, 1e-3);
      });

      it('select an area in date mode', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
          el.selectionType = mode as any;
          // Enable date mode, the underlying data should stay the same.
          el.domain = 'date';
        });
        await element.updateComplete;
        await chartReady(() => {
          element.dataframe = df;
          return element;
        });

        const header = df.header!,
          start = 3,
          end = 9;
        element.Select(header![start]!, header![end]!);
        assert.approximately(element.selectedValueRange!.begin, header[start]!.timestamp, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.timestamp, 1e-3);
      });
    })
  );
});
