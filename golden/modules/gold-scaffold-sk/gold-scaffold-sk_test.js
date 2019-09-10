import './index.js'

describe('gold-scaffold-sk', () => {

  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  afterEach(function() {
    container.innerHTML = '';
  });

  // calls the test callback with one element 'ele', a created <swarming-app>.
  // We can't put the describes inside the whenDefined callback because
  // that doesn't work on Firefox (and possibly other places).
  function createElement(test) {
    return window.customElements.whenDefined('gold-scaffold-sk').then(() => {
      container.innerHTML = `
          <gold-scaffold-sk testing_offline>
            <div>content</div>
          </sgold-scaffold-sk>`;
      expect(container.firstElementChild).to.not.be.null;
      test(container.firstElementChild);
    });
  }

  //===============TESTS START====================================

  describe('spinner and busy property', () => {
    it('becomes busy while there are tasks to be done', () => {
      return createElement((ele) => {
        expect(ele.busy).to.equal(false);
        ele.addBusyTasks(2);
        expect(ele.busy).to.equal(true);
        ele.finishedTask();
        expect(ele.busy).to.equal(true);
        ele.finishedTask();
        expect(ele.busy).to.equal(false);
      });
    });

    it('keeps spinner active while busy', () => {
      return createElement((ele) => {
        const spinner = ele.querySelector('header spinner-sk');
        expect(spinner.active).to.equal(false);
        ele.addBusyTasks(2);
        expect(spinner.active).to.equal(true);
        ele.finishedTask();
        expect(spinner.active).to.equal(true);
        ele.finishedTask();
        expect(spinner.active).to.equal(false);
      });
    });

    it('emits a busy-end task when tasks finished', function(done) {
      createElement((ele) => {
        ele.addEventListener('busy-end', (e) => {
          e.stopPropagation();
          expect(ele.busy).to.equal(false);
          done();
        });
        ele.addBusyTasks(1);

        setTimeout(()=>{
          ele.finishedTask();
        }, 10);
      });
    });
  }); // end describe('spinner and busy property')

});