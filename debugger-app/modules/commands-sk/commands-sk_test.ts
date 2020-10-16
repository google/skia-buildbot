import './index';
import { CommandsSk, CommandsSkMovePositionEventDetail } from './commands-sk';

import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { SkpJsonCommandList } from '../debugger';


const testData: SkpJsonCommandList = {
  commands: [
    { // 0
      command: 'Save',
      shortDesc: 'the trees',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 1
      command: 'DrawAnnotation',
      shortDesc: '',
      key: 'What kind of SKP is this anyways',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 2
      command: 'ClipRect',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 3
      command: 'DrawImageRect',
      shortDesc: 'A picture of a corgy',
      key: '',
      imageIndex: 3,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 4
      command: 'Save',
      shortDesc: 'your money',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 5
      command: 'ClipRRect',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 6
      command: 'DrawTextBlob',
      shortDesc: 'user was panned for this boast',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: {
        Ops: [
          {
            'Name': 'North Korelana is Best Korelana',
            'ClientID': 1,
            'OpsTaskID': 2,
            'ChildID': 3,
          }
        ],
      },
    },
    { // 7
      command: 'DrawImageRectLayer',
      shortDesc: '',
      key: '',
      imageIndex: 0,
      layerNodeId: 20, // TODO(nifong): make this actually refer to something once layer parsing added
      auditTrail: { Ops: [] },
    },
    { // 8
      command: 'Restore',
      shortDesc: 'your money',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
    { // 9
      command: 'Restore',
      shortDesc: 'the trees',
      key: '',
      imageIndex: 0,
      layerNodeId: 0,
      auditTrail: { Ops: [] },
    },
  ],
};

describe('commands-sk', () => {
  const newInstance = setUpElementUnderTest<CommandsSk>('commands-sk');

  let element: CommandsSk;
  beforeEach(() => {
    element = newInstance((el: CommandsSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('element behavior', () => {
    it('Can process a list of commands', () => {
      element.processCommands(testData);
      expect(element.count).to.equal(10);
      expect(element.countFiltered).to.equal(9); // default filter excludes DrawAnnotation, so one less command
      expect(element.position).to.equal(9); // last item in list
    });

    it('can process a second command list after loading a first', () => {
      element.processCommands(testData);
      element.range = [4, 9];

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
      element.processCommands(newData);
      expect(element.count).to.equal(3);
      expect(element.countFiltered).to.equal(3);
      expect(element.position).to.equal(2);
      // confirm filters gone
      expect((document.getElementById('rangelo') as HTMLInputElement).value).to.equal('0');
      expect((document.getElementById('rangehi') as HTMLInputElement).value).to.equal('2'); // the highest index
      expect((document.getElementById('text-filter') as HTMLInputElement).value).to.equal('!DrawAnnotation'); // We don't intend to clear this.
    });
  });

  it('Can apply a range filter by setting range attribute', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.range = [2, 6];
    
    expect(element.count).to.equal(10); // this should never change
    expect(element.countFiltered).to.equal(5);
    expect(element.position).to.equal(6);
    expect(element.filtered).to.eql([2, 3, 4, 5, 6]); // chai deep equals
  });

  it('Can apply a range filter by clicking the zoom button on one of the ops', () => {
    element.clearFilter();
    element.processCommands(testData);

    // a div containing a save op with at matching restore at op 8.
    const opDiv = document.getElementById('op-4') as HTMLElement;
    // CommandsSk is supposed to find it and remember the range during processCommands. if there is no button, that part failed.
    (opDiv.querySelector('button') as HTMLButtonElement).click();
    
    expect(element.countFiltered).to.equal(5);
    expect(element.position).to.equal(8);
    expect(element.filtered).to.eql([4, 5, 6, 7, 8]);
  });
 
  it('Can apply a positive text filter (ClipRect cliprrect)', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "ClipRect cliprrect";
    
    expect(element.countFiltered).to.equal(2);
    expect(element.position).to.equal(5); // the last item to pass the filter, op 5, cliprrect
    expect(element.filtered).to.eql([2, 5]);
  });
 
  it('Can apply a negative text filter (!Restore Save)', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "!Restore Save";
    
    expect(element.countFiltered).to.equal(6);
    expect(element.position).to.equal(7);
    expect(element.filtered).to.eql([1, 2, 3, 5, 6, 7]);
  });
 
  // because theres a token that doesn't match a command name, it should be interpreted as a free text search
  it('Can apply a free text search filter (money)', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "money";
    
    expect(element.countFiltered).to.equal(2);
    expect(element.position).to.equal(8);
    expect(element.filtered).to.eql([4, 8]);
  });

  it('Can apply a range filter while a positive text filter is applied', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "Save"; // there's save at ops 0 and 4
    element.range = [2,9];
    
    expect(element.countFiltered).to.equal(1); // only one op, the save at position 4, satisfies both filters
    expect(element.position).to.equal(4);
    expect(element.filtered).to.eql([4]);
  });

  it('Can apply a range filter while a negative text filter is applied', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "!Save"; // there's saves at ops 0 and 4
    element.range = [2,9];
    
    expect(element.countFiltered).to.equal(7); // only one op, the save at position 4, satisfies both filters
    expect(element.position).to.equal(9);
    expect(element.filtered).to.eql([2, 3, 5, 6, 7, 8, 9]);
  });

  it('Can apply a range filter while a free text filter is applied', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "trees"; // there's matches at ops 0 and 9
    element.range = [0,2];
    
    expect(element.countFiltered).to.equal(1);
    expect(element.position).to.equal(0);
    expect(element.filtered).to.eql([0]);
  });

  it('Can click clear filter button while both types of filter apply.', () => {
    element.clearFilter();
    element.processCommands(testData);

    element.textFilter = "trees"; // there's matches at ops 0 and 9
    element.range = [0,2];

    (document.getElementById('clear-filter-button') as HTMLButtonElement).click();

    // confirm filters gone
    expect((document.getElementById('rangelo') as HTMLInputElement).value).to.equal('0');
    expect((document.getElementById('rangehi') as HTMLInputElement).value).to.equal('9'); // the highest index
    expect((document.getElementById('text-filter') as HTMLInputElement).value).to.equal('');
    // And applied
    expect(element.countFiltered).to.equal(10);
    expect(element.position).to.equal(0); // Does not change, also tested below in different circumstances
    expect(element.filtered).to.eql([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  });

  it('Can put playback on an arbitrary command by clicking the <summary> element', () => {
    element.processCommands(testData);

    const opDiv = document.getElementById('op-5') as HTMLElement; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();
    
    expect(element.countFiltered).to.equal(9);
    expect(element.position).to.equal(5);
  });

  it('Can apply a filter that would not exclude the currently selected item and it will still be selected.', () => {
    element.clearFilter();
    element.processCommands(testData);

    // select item 6, DrawTextBlob
    const opDiv = document.getElementById('op-6') as HTMLElement; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();

    element.textFilter = "!Save Restore";
    
    expect(element.position).to.equal(6);
  });

  it('Can apply a filter that would exclude the currently selected item and it will select the last item', () => {
    element.clearFilter();
    element.processCommands(testData);

    // select item 6, DrawTextBlob
    const opDiv = document.getElementById('op-6') as HTMLElement; // ClipRRect
    (opDiv.querySelector('summary') as HTMLElement).click();

    element.textFilter = "!DrawTextBlob";
    
    expect(element.position).to.equal(9);
  });

  it('playback loops around at the end of a filtered list', async () => {
    element.clearFilter();
    element.processCommands(testData);

    element.range = [4, 9];
    expect(element.position).to.equal(9);

    // set up event promise
    let ep = eventPromise<CustomEvent<CommandsSkMovePositionEventDetail>>('move-position', 200);

    // click the play button
    (document.getElementById('play-button') as HTMLButtonElement).click();

    expect((await ep).detail.position).to.equal(4); 
  });

});
