// Enter debugger after running each test to allow debugging visually.
// Run Karma with --no-single-run and change "it" to "it.only" for one of the
// tests below. You may need to zoom in to see the marker.
var enableVisualDebug = false;
// Id for visual check. Global to make it easier to set in the JS debugger.
if (enableVisualDebug) { var id = '#a'; }

describe('sk.elePos',
  function() {
    var container;

    // This is not a test function. Intended for debugging visually.
    function visualDebug() {
      var style = document.createElement('style');
      style.innerHTML = '.marker {' +
          '  -webkit-animation-duration: 2s; ' +
          '  -webkit-animation-name: blink; ' +
          '  -webkit-animation-iteration-count: infinite; ' +
          '  position: fixed; ' +
          '  width: 1px; ' +
          '  height: 1px; ' +
          '  background-color: red; ' +
          '  margin: 0px; ' +
          '  padding: 0px; ' +
          '  border: 0px; ' +
          '  z-index: 1000' +
          '} ' +
          '@-webkit-keyframes blink {' +
          '  from { ' +
          '    background-color: red; ' +
          '  } ' +
          '  50% { ' +
          '    background-color: black; ' +
          '  } ' +
          '  to { ' +
          '    background-color: red; ' +
          '  } ' +
          '}';
      document.body.appendChild(style);
      var marker = document.createElement('p');
      marker.innerHTML = '&nbsp;';
      marker.className = "marker"
      setInterval(function() {
        var ele = $$$(id, container);
        if (ele) {
          var pos = sk.elePos(ele);
          marker.style.left = pos.x + 'px';
          marker.style.top = pos.y + 'px';
        }
      }, 500);
      document.body.appendChild(marker);
      assert.fail('Check browser window.');
    }

    afterEach(function () {
      if (enableVisualDebug) {
        visualDebug();
      }
      if (container) {
        document.body.removeChild(container);
      }
    });

    // Assert that sk.elePos($$$(id, container)) is offset from
    // sk.elePos(container) by (x, y).
    function assertRelative(id, x, y) {
      if (!container) {
        assert.fail('container not set');
      }
      var containerPos = sk.elePos(container);
      var actual = sk.elePos($$$(id, container));
      assert.equal(actual.x - containerPos.x, x,
                   "Expected x-axis position of '" + id + "' to be " +
                       containerPos.x + " + " + x + " = " +
                       (containerPos.x + x) + ", but elePos returned " +
                       actual.x);
      assert.equal(actual.y - containerPos.y, y,
                   "Expected y-axis position of '" + id + "' to be " +
                       containerPos.y + " + " + y + " = " +
                       (containerPos.y + y) + ", but elePos returned " +
                       actual.y);
    }

    function testPaddingBorderPosition() {
      // Add an HTML tree to the document.
      container = document.createElement('div');
      container.innerHTML =
          '<div id=a style="padding: 2px; border: 3px solid;">' +
          '  <div id=aa style="padding: 5px; border: 7px solid;">' +
          '    <p id=aaa style="position: absolute; left: 11px; top: 13px; ' +
          '                     margin: 0px;">aaa</p>' +
          '    <p id=aab style="padding: 17px; border: 19px solid; ' +
          '                     margin: 0px;">aab</p>' +
          '  </div>' +
          '  <div id=ab style="position: fixed; left: 23px; top: 29px;">' +
          '    <div id=aba style="padding: 31px; border: 37px solid;">' +
          '      <p id=abaa style="position: absolute; left: 41px; ' +
          '                        top: 43px; margin: 0px;">abaa</p>' +
          '      <p id=abab style="margin: 0px;">abab</p>' +
          '    </div>' +
          '  </div>' +
          '  <div id=ac style="position: relative; left: 47px; top: 53px;">' +
          '    <p id=aca style="position: absolute; left: 59px; top: 61px; ' +
          '                     margin: 0px;">aca</p>' +
          '    <p id=acb style="margin: 0px;">acb</p>' +
          '  </div>' +
          '  <p id=ad style="margin: 0px;">ad</p>' +
          '</div>';
      document.body.appendChild(container);
      // a: should be at top-left of container.
      assertRelative('#a', 0, 0);
      // aa: offset by padding/border of a, so both top and left shifted by
      // 2+3=5px.
      assertRelative('#aa', 5, 5);
      // aaa: all parents are statically-positioned, so absolute positioning
      // should be relative to the document.
      assert.deepEqual(sk.elePos($$$('#aaa', container)), {x: 11, y: 13});
      // aab: offset by padding/border of a and aa. Not affected by aaa due to
      // absolute positioning. Both top and left shifted by 2+3+5+7=17.
      assertRelative('#aab', 17, 17);
      // ab: fixed positioning is relative to the document.
      assert.deepEqual(sk.elePos($$$('#ab', container)), {x: 23, y: 29});
      // aba: since ab has no padding/border, should at the same location as ab.
      assert.deepEqual(sk.elePos($$$('#aba', container)), {x: 23, y: 29});
      // abaa: absolute positioning is relative to ab; 23+41=64, 29+43=72.
      assert.deepEqual(sk.elePos($$$('#abaa', container)), {x: 64, y: 72});
      // abab: offset by padding/border of aba, so both top and left shifted by
      // 31+37=68. Not affected by abaa due to absolute positioning. Positioned
      // relative to aba; 23+68=91, 29+68=97.
      assert.deepEqual(sk.elePos($$$('#abab', container)), {x: 91, y: 97});
      // ac is offset downward by aa, but ab does not affect the position of ac
      // due to fixed positioning.
      // Total height is clientHeight (which includes padding) plus top and
      // bottom border.
      var aaTotalHeight = $$$('#aa', container).clientHeight + 7 * 2;
      // ac: offset by padding/border of a (2+3=5px) and height of aa, then
      // shifted by relative positioning. 5+47=52, 5+53=58.
      assertRelative('#ac', 52, 58 + aaTotalHeight);
      // aca: absolute positioning is relative to ac; 52+59=111, 58+61=119.
      assertRelative('#aca', 111, 119 + aaTotalHeight);
      // acb: not affected by aca due to absolute positioning. ac does not have
      // padding/border, so acb is at the same location.
      assertRelative('#acb', 52, 58 + aaTotalHeight);
      // ad: offset by padding/border of a (2+3=5px), and offset downward by aa
      // and ac, but not affected by ab due to fixed positioning.
      // Total height of ac is clientHeight because it does not have a border.
      var acTotalHeight = $$$('#ac', container).clientHeight;
      assertRelative('#ad', 5, 5 + aaTotalHeight + acTotalHeight);
    }

    it('should give the location of an element with padding, border, and ' +
        'position CSS', testPaddingBorderPosition);

    function testMargin() {
      // Add an HTML tree to the document.
      container = document.createElement('div');
      container.innerHTML =
          '<div id=b style="margin: 2px;">' +
          '  <p id=ba style="position: absolute; left: 3px; top: 5px; ' +
          '                  margin: 7px;">ba</p>' +
          '  <p id=bb style="margin: 11px;">bb</p>' +
          '</div>' +
          '<div id=c style="position: fixed; left: 13px; top: 17px; ' +
          '                 margin: 19px;">' +
          '  <div id=ca style="margin: 23px;">' +
          '    <p id=caa style="margin: 10px;">caa</p>' +
          '  </div>' +
          '</div>' +
          '<div id=d style="position: relative; left: 43px; top: 47px; ' +
          '                 margin: 53px;">' +
          '  <p id=da style="margin: 71px">da</p>' +
          '</div>' +
          '<p id=e style="margin: 50px">e</p>';
      container.style.position = 'absolute';
      container.style.top = '50px;';
      container.style.left = '50px;';
      document.body.appendChild(container);
      // b: top margin of bb collapses outside b, causing top of b to be shifted
      // down 11 rather than 2. Left is shifted by margin of 2.
      assertRelative('#b', 2, 11);
      // ba: positioned relative to container, shifted by margin; 3+7=10,
      // 5+7=12.
      assertRelative('#ba', 10, 12);
      // bb: offset by margin of bb and left margin of b. Not affected by ba due
      // to absolute positioning. Not affected by top margin of b because of
      // margin collapsing. 11+2=13
      assertRelative('#bb', 13, 11);
      // c: fixed positioning is relative to the document, shifted by margin;
      // 13+19=32, 17+19=36.
      assert.deepEqual(sk.elePos($$$('#c', container)), {x: 32, y: 36});
      // ca: Offset from c by the margins of c and ca; no margin collapsing due
      // to fixed positioning of c; 13+19+23=55, 17+19+23=59.
      assert.deepEqual(sk.elePos($$$('#ca', container)), {x: 55, y: 59});
      // caa: offset from c by margins of c, ca, and caa modulo margin
      // collapsing. Left is at 13+19+23+10=65. Top margin of caa collapses with
      // ca so that top is at 17+19+23=59.
      assert.deepEqual(sk.elePos($$$('#caa', container)), {x: 65, y: 59});
      // d is offset downward by b, but c does not affect the position of d due
      // to fixed positioning. Margins of b, d, and da are collapsed so that the
      // top of d is 71 below the bottom of b. Additionally d is positioned
      // relatively, so is shifted by (43, 47) from its static position.
      // Therefore top of d relative to container is
      // (collapsed top margin of b)+(clientHeight of b)+
      //   (collapsed margins of b, d, and da)+(relative position of d) =
      //   11+bClientHeight+71+47 = 129+bClientHeight.
      // Left of d relative to container is 43+53=96.
      var bClientHeight = $$$('#b', container).clientHeight;
      assertRelative('#d', 96, 129 + bClientHeight);
      // da: Left margin shifts da by 71 from left of d. Since margins of d and
      // da collapsed, top of da is the same as top of d.
      assertRelative('#da', 167, 129 + bClientHeight);
      // e: Shifted downward from d's static position by the clientHeight of d
      // and the collapsed margins of da and e, i.e. 71. Top is at
      // (collapsed top margin of b)+(clientHeight of b)+
      //   (collapsed margins of b, d, and da)+(clientHeight of d)+
      //   (collapsed margins of da and e) =
      //   11+bClientHeight+71+dClientHeight+71 =
      //   153+bClientHeight+dClientHeight
      var dClientHeight = $$$('#d', container).clientHeight;
      assertRelative('#e', 50, 153 + bClientHeight + dClientHeight);
    }

    it('should give the location of an element with collapsed margins',
       testMargin);

    function testScrolling() {
      // Add an HTML tree to the document.
      container = document.createElement('div');
      container.innerHTML =
          '<div id=f style="width: 100px; height: 100px; overflow: scroll;">' +
          '  <p id=fa style="position: absolute; left: 2px; top: 3px; ' +
          '                  margin: 0px;">fa</p>' +
          '  <p id=fb style="margin: 0px;">fb</p>' +
          '  <p id=fc style="position: fixed; left: 5px; top: 7px; ' +
          '                  margin: 0px;">fc</p>' +
          '  <p id=fd style="position: relative; left: 11px; top: 13px; ' +
          '                  margin: 0px;">fd</p>' +
          '  <p style="width: 1000px; height: 1000px">&nbsp;</p>' +
          '</div>' +
          '<p style="width: 1000px; height: 1000px">&nbsp;</p>';
      document.body.appendChild(container);
      var f = $$$('#f', container);
      f.scrollLeft = 17;
      f.scrollTop = 19;
      document.body.scrollLeft = 0;
      document.body.scrollTop = 0;
      // Just in case we scrolled more than the max.
      var fScrollLeft = f.scrollLeft;
      var fScrollTop = f.scrollTop;
      // f: at the same location as container.
      assertRelative('#f', 0, 0);
      // fa: all parents are statically-positioned, so absolute positioning
      // should be relative to the document.
      assert.deepEqual(sk.elePos($$$('#fa', container)), {x: 2, y: 3});
      // fb: offset from f by scrolling.
      assertRelative('#fb', -fScrollLeft, -fScrollTop);
      // fc: fixed positioning is relative to the document.
      assert.deepEqual(sk.elePos($$$('#fc', container)), {x: 5, y: 7});
      // fd is offset downward by fb, but fa and fc do not affect the position
      // of fd.
      var fbClientHeight = $$$('#fb', container).clientHeight;
      // fd: offset from f by (11, 13), by scrolling, and by fbClientHeight.
      assertRelative('#fd', 11 - fScrollLeft, 13 - fScrollTop + fbClientHeight);
    }

    it('should give the location of an element that has been scrolled',
       testScrolling);
  }
);
