import './index';
import { CommandsSk, CommandsSkMovePositionEventDetail } from './commands-sk';

import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { SkpJsonCommandList } from '../debugger';
import { testData } from './test-data';


describe('commands-sk', () => {
  const newInstance = setUpElementUnderTest<CommandsSk>('commands-sk');

  let commandsSk: CommandsSk;
  beforeEach(() => {
    commandsSk = newInstance((el: CommandsSk) => {});
  });

  it('can process a list of commands', () => {
    commandsSk.processCommands(testData);
    expect(commandsSk.count).to.equal(10);
    // default filter excludes DrawAnnotation, so one less command
    expect(commandsSk.countFiltered).to.equal(9);
    expect(commandsSk.position).to.equal(9); // last item in list
  });

  it('can process a second command list after loading a first', () => {
    commandsSk.processCommands(testData);
    commandsSk.range = [4, 9];

    const newData: SkpJsonCommandList = {
      commands: [
        {
          command: 'DrawRect',
          shortDesc: '',
          key: '',
          imageIndex: 0,
          layerNodeId: 0,
          auditTrail: { Ops: [] },
        },
        {
          command: 'DrawOval',
          shortDesc: '',
          key: '',
          imageIndex: 0,
          layerNodeId: 0,
          auditTrail: { Ops: [] },
        },
        {
          command: 'DrawPaint',
          shortDesc: '',
          key: '',
          imageIndex: 0,
          layerNodeId: 0,
          auditTrail: { Ops: [] },
        },
      ],
    };
    commandsSk.processCommands(newData);
    expect(commandsSk.count).to.equal(3);
    expect(commandsSk.countFiltered).to.equal(3);
    expect(commandsSk.position).to.equal(2);
    // confirm filters gone
    expect(commandsSk.querySelector<HTMLInputElement>('#rangelo')!.value)
      .to.equal('0');
    expect(commandsSk.querySelector<HTMLInputElement>('#rangehi')!.value)
      .to.equal('2'); // the highest index
    expect(commandsSk.querySelector<HTMLInputElement>('#text-filter')!.value)
      .to.equal('!DrawAnnotation'); // We don't intend to clear this.
  });

  it('can apply a range filter by setting range attribute', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.range = [2, 6];

    expect(commandsSk.count).to.equal(10); // this should never change
    expect(commandsSk.countFiltered).to.equal(5);
    expect(commandsSk.position).to.equal(6);
    expect(commandsSk.filtered).to.eql([2, 3, 4, 5, 6]); // chai deep equals
  });

  it('can apply a range filter by clicking the zoom button on one of the ops', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    // a div containing a save op with at matching restore at op 8.
    const opDiv = commandsSk.querySelector<HTMLElement>('#op-4')!;
    // CommandsSk is supposed to find it and remember the range during processCommands.
    // If there is no button, that part failed.
    (opDiv.querySelector('button') as HTMLButtonElement).click();

    expect(commandsSk.countFiltered).to.equal(5);
    expect(commandsSk.position).to.equal(8);
    expect(commandsSk.filtered).to.eql([4, 5, 6, 7, 8]);
  });

  it('can apply a positive text filter (ClipRect cliprrect)', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "ClipRect cliprrect";

    expect(commandsSk.countFiltered).to.equal(2);
    // the last item to pass the filter, op 5, cliprrect
    expect(commandsSk.position).to.equal(5);
    expect(commandsSk.filtered).to.eql([2, 5]);
  });

  it('can apply a negative text filter (!Restore Save)', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "!Restore Save";

    expect(commandsSk.countFiltered).to.equal(6);
    expect(commandsSk.position).to.equal(7);
    expect(commandsSk.filtered).to.eql([1, 2, 3, 5, 6, 7]);
  });

  // because theres a token that doesn't match a command name, it should be interpreted
  // as a free text search
  it('Can apply a free text search filter (money)', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "money";

    expect(commandsSk.countFiltered).to.equal(2);
    expect(commandsSk.position).to.equal(8);
    expect(commandsSk.filtered).to.eql([4, 8]);
  });

  it('can apply a range filter while a positive text filter is applied', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "Save"; // there's save at ops 0 and 4
    commandsSk.range = [2,9];

    // only one op, the save at position 4, satisfies both filters
    expect(commandsSk.countFiltered).to.equal(1);
    expect(commandsSk.position).to.equal(4);
    expect(commandsSk.filtered).to.eql([4]);
  });

  it('can apply a range filter while a negative text filter is applied', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "!Save"; // there's saves at ops 0 and 4
    commandsSk.range = [2,9];

    // only one op, the save at position 4, satisfies both filters
    expect(commandsSk.countFiltered).to.equal(7);
    expect(commandsSk.position).to.equal(9);
    expect(commandsSk.filtered).to.eql([2, 3, 5, 6, 7, 8, 9]);
  });

  it('can apply a range filter while a free text filter is applied', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "trees"; // there's matches at ops 0 and 9
    commandsSk.range = [0,2];

    expect(commandsSk.countFiltered).to.equal(1);
    expect(commandsSk.position).to.equal(0);
    expect(commandsSk.filtered).to.eql([0]);
  });

  it('can click clear filter button while both types of filter apply.', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.textFilter = "trees"; // there's matches at ops 0 and 9
    commandsSk.range = [0,2];

    commandsSk.querySelector<HTMLButtonElement>('#clear-filter-button')!.click()

    // confirm filters gone
    expect(commandsSk.querySelector<HTMLInputElement>('#rangelo')!.value)
      .to.equal('0');
    expect(commandsSk.querySelector<HTMLInputElement>('#rangehi')!.value)
      .to.equal('9'); // the highest index
    expect(commandsSk.querySelector<HTMLInputElement>('#text-filter')!.value)
      .to.equal('');
    // And applied
    expect(commandsSk.countFiltered).to.equal(10);
    // Does not change, also tested below in different circumstances
    expect(commandsSk.position).to.equal(0);
    expect(commandsSk.filtered).to.eql([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  });

  it('can put playback on an arbitrary command by clicking the <summary> element', () => {
    commandsSk.processCommands(testData);

    const opDiv = commandsSk.querySelector<HTMLElement>('#op-5')!; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();

    expect(commandsSk.countFiltered).to.equal(9);
    expect(commandsSk.position).to.equal(5);
  });

  it('can apply a filter without clobbering selection', () => {
    // Apply a filter that would not exclude the currently selected item and confirm
    // it is still selected.
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    // select item 6, DrawTextBlob
    const opDiv = commandsSk.querySelector<HTMLElement>('#op-6')!; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();

    commandsSk.textFilter = "!Save Restore";

    expect(commandsSk.position).to.equal(6);
  });

  it('can apply a filter that removes selection and alter it correctly.', () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    // select item 6, DrawTextBlob
    const opDiv = commandsSk.querySelector<HTMLElement>('#op-6')!; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();

    commandsSk.textFilter = "!DrawTextBlob";

    expect(commandsSk.position).to.equal(9);
  });

  it('playback loops around at the end of a filtered list', async () => {
    commandsSk.clearFilter();
    commandsSk.processCommands(testData);

    commandsSk.range = [4, 9];
    expect(commandsSk.position).to.equal(9);

    // set up event promise
    let ep = eventPromise<CustomEvent<CommandsSkMovePositionEventDetail>>(
      'move-position', 200);

    // click the play button
    commandsSk.querySelector<HTMLButtonElement>('#play-button')!.click();

    expect((await ep).detail.position).to.equal(4);
  });

});
