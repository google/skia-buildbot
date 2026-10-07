import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  ActiveFailureClass,
  FailureClass,
  flattenFailureClass,
  FailureClassesSk,
} from './failure-classes-sk';

describe('failure-classes-sk', () => {
  const newInstance = setUpElementUnderTest<FailureClassesSk>('failure-classes-sk');

  afterEach(() => {
    fetchMock.restore();
  });

  const testClasses: { failureClass: FailureClass; taskIds: string[] }[] = [
    {
      failureClass: {
        id: 'class-1',
        updates: [
          {
            timestamp: '2026-09-30T10:00:00Z',
            user: 'autogardener',
            analysis: 'Compiler error in SkCanvas.cpp',
            errorMessage: 'SkCanvas.cpp:42: undefined symbol foo',
          },
        ],
      },
      taskIds: ['task-1', 'task-2'],
    },
    {
      failureClass: {
        id: 'class-2',
        updates: [
          {
            timestamp: '2026-09-30T10:00:00Z',
            user: 'autogardener',
            analysis: 'Timeout running dm on Android GPU',
            errorMessage: 'Step dm timed out after 3600s',
          },
        ],
      },
      taskIds: ['task-3'],
    },
  ];

  it('fetches and displays failure classes when taskIds are set', async () => {
    fetchMock.postOnce('/json/failure-classes', testClasses);
    const el = newInstance();
    el.taskIds = ['task-1', 'task-2', 'task-3'];
    await fetchMock.flush(true);

    const items = $('.failure-class-item', el);
    expect(items).to.have.length(2);
    expect($$('.analysis', items[0])!.textContent).to.equal('Compiler error in SkCanvas.cpp');
    expect($$('.value', items[0])!.textContent!.trim()).to.equal('2');
    expect($$('.analysis', items[1])!.textContent).to.equal('Timeout running dm on Android GPU');
    expect($$('.value', items[1])!.textContent!.trim()).to.equal('1');
  });

  it('dispatches highlight-tasks on mouseenter and mouseleave', async () => {
    fetchMock.postOnce('/json/failure-classes', testClasses);
    const el = newInstance();
    el.taskIds = ['task-1', 'task-2', 'task-3'];
    await fetchMock.flush(true);

    const events: string[][] = [];
    el.addEventListener('highlight-tasks', (e: Event) => {
      events.push((e as CustomEvent).detail.taskIds);
    });

    const firstItem = $$('.failure-class-item', el) as HTMLElement;
    firstItem.dispatchEvent(new MouseEvent('mouseenter'));
    expect(events).to.deep.equal([['task-1', 'task-2']]);
    expect(firstItem.classList.contains('hovered')).to.be.true;

    firstItem.dispatchEvent(new MouseEvent('mouseleave'));
    expect(events).to.deep.equal([['task-1', 'task-2'], []]);
    expect(firstItem.classList.contains('hovered')).to.be.false;
  });

  it('highlights matching class and dispatches highlight-tasks when setHoveredTask is called', async () => {
    fetchMock.postOnce('/json/failure-classes', testClasses);
    const el = newInstance();
    el.taskIds = ['task-1', 'task-2', 'task-3'];
    await fetchMock.flush(true);

    const events: string[][] = [];
    el.addEventListener('highlight-tasks', (e: Event) => {
      events.push((e as CustomEvent).detail.taskIds);
    });

    el.setHoveredTask('task-2', true);
    expect(events).to.deep.equal([['task-1', 'task-2']]);
    const firstItem = $$('.failure-class-item', el) as HTMLElement;
    expect(firstItem.classList.contains('hovered')).to.be.true;

    el.setHoveredTask('task-2', false);
    expect(events).to.deep.equal([['task-1', 'task-2'], []]);
    expect(firstItem.classList.contains('hovered')).to.be.false;
  });

  it('dispatches select-failure-class when clicked', async () => {
    fetchMock.postOnce('/json/failure-classes', testClasses);
    const el = newInstance();
    el.taskIds = ['task-1', 'task-2', 'task-3'];
    await fetchMock.flush(true);

    let selected: ActiveFailureClass | null = null;
    el.addEventListener('select-failure-class', (e: Event) => {
      selected = (e as CustomEvent<ActiveFailureClass>).detail;
    });

    const firstItem = $$('.failure-class-item', el) as HTMLElement;
    firstItem.click();

    expect(selected).to.deep.equal({
      failureClass: flattenFailureClass(testClasses[0].failureClass),
      taskIds: testClasses[0].taskIds,
    });
  });

  it('flattens updates in chronological order', () => {
    const flattened = flattenFailureClass({
      id: 'class-1',
      updates: [
        {
          timestamp: '2026-09-30T10:00:00Z',
          user: 'autogardener',
          title: 'Initial title',
          analysis: 'Initial analysis',
          errorMessage: 'SkCanvas.cpp:42: undefined symbol foo\nextra noise',
          flaky: true,
          culprits: ['commit-a', 'commit-b'],
          resolvedBy: null,
          bugs: null,
        },
        {
          timestamp: '2026-09-30T10:05:00Z',
          user: 'dev@google.com',
          message: 'Not flaky, introduced by commit-b',
          title: 'Updated title',
          analysis: 'Compiler error in SkCanvas.cpp',
          errorMessage: 'SkCanvas.cpp:42: undefined symbol foo',
          flaky: false,
          culprits: ['commit-b'],
          bugs: ['b/12345'],
          duplicateOf: 'class-0',
          duplicates: ['class-2', 'class-3'],
        },
        {
          timestamp: '2026-09-30T10:15:00Z',
          user: 'autogardener',
          title: '',
          resolvedBy: ['commit-d'],
          bugs: [],
          duplicateOf: '',
        },
      ],
    });

    expect(flattened.title).to.equal('');
    expect(flattened.analysis).to.equal('Compiler error in SkCanvas.cpp');
    expect(flattened.errorMessage).to.equal('SkCanvas.cpp:42: undefined symbol foo');
    expect(flattened.flaky).to.be.false;
    expect(flattened.culprits).to.deep.equal(['commit-b']);
    expect(flattened.resolvedBy).to.deep.equal(['commit-d']);
    expect(flattened.bugs).to.deep.equal([]);
    expect(flattened.duplicateOf).to.equal('');
    expect(flattened.duplicates).to.deep.equal(['class-2', 'class-3']);
  });

  it('merges duplicate failure classes into canonical class on the client', async () => {
    const classesWithDuplicate: { failureClass: FailureClass; taskIds: string[] }[] = [
      {
        failureClass: {
          id: 'class-dup',
          updates: [
            {
              timestamp: '2026-09-30T10:00:00Z',
              user: 'autogardener',
              analysis: 'Duplicate failure class',
              duplicateOf: 'class-canonical',
            },
          ],
        },
        taskIds: ['task-1', 'task-2'],
      },
      {
        failureClass: {
          id: 'class-canonical',
          updates: [
            {
              timestamp: '2026-09-30T10:00:00Z',
              user: 'autogardener',
              analysis: 'Canonical failure class',
              duplicates: ['class-dup'],
            },
          ],
        },
        taskIds: ['task-3'],
      },
    ];

    fetchMock.postOnce('/json/failure-classes', classesWithDuplicate);
    const el = newInstance();
    el.taskIds = ['task-1', 'task-2', 'task-3'];
    await fetchMock.flush(true);

    const items = $('.failure-class-item', el);
    expect(items).to.have.length(1);
    expect($$('.analysis', items[0])!.textContent).to.equal('Canonical failure class');
    expect($$('.value', items[0])!.textContent!.trim()).to.equal('3');

    const events: string[][] = [];
    el.addEventListener('highlight-tasks', (e: Event) => {
      events.push((e as CustomEvent).detail.taskIds);
    });
    el.setHoveredTask('task-1', true);
    expect(events).to.deep.equal([['task-1', 'task-2', 'task-3']]);
  });
});
