This is the top level design for how the home page of skiaperf will be broken into a set of Polymer elments
and a JS object. Once this is fully implemented this comtent will be moved over to res/js/logic.js.

Navigation
  Object that registers for events from plot, note, and query and routes them to the appropriate element.
  Also registers for events from all the buttons and acts on them (e.g. Add Calculated Trace).

  Member variables
  - tiles
  - scale
  - commitInfo
  - ticks
  - skps
  - stepIndex
  - a reference to each element below.


  <plot-sk>
    Events
    - selected
    - highlighted
    Methods
    - addTrace()
    - removeTrace(id)
    - remove(id)
    - removeAllBut(key, value)
    - setBackgroundInfo(ticks, skps)
    - setStepIndex(index)
    - hightlightGroup(key, value)

  <highlightbar-sk>
    Attributes:
    - name
    - value

  <trace-detail-sk> aka note.
    Events
    - only
    - remove
    - group
    Methods
    - blank()
    - displayRange(begin, end)
    - setParams(id, params)

  <query-sk>
    Events
    - change
    Attributes
    - scale
    - tiles
