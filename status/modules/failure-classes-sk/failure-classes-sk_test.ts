import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ActiveFailureClass, FailureClassesSk } from './failure-classes-sk';

describe('failure-classes-sk', () => {
  const newInstance = setUpElementUnderTest<FailureClassesSk>('failure-classes-sk');

  afterEach(() => {
    fetchMock.restore();
  });

  const testClasses: ActiveFailureClass[] = [
    {
      failureClass: {
        id: 'class-1',
        analysis: 'Compiler error in SkCanvas.cpp',
        errorMessage: 'SkCanvas.cpp:42: undefined symbol foo',
      },
      taskIds: ['task-1', 'task-2'],
    },
    {
      failureClass: {
        id: 'class-2',
        analysis: 'Timeout running dm on Android GPU',
        errorMessage: 'Step dm timed out after 3600s',
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

    expect(selected).to.deep.equal(testClasses[0]);
  });
});
