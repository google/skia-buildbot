// Copyright (c) 2019 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
console.log("HERE HERE111111111111111");
asdfasdf

debugger;
console.log("HERE HERE");

(function() {

  // Doing a conditional write allows selecting, copying and paste to work
  // instead of the selection constantly going away.
  function overwriteIfDifferent(elem, str) {
    if (elem.innerHTML !== str) {
      elem.innerHTML = str;
    }
  }

  console.log("HERE HERE");

  window.addEventListener("WebComponentsReady", function(e) {
    console.log("HERE");
    // Find all hashtags on the page and turn them into links.
    var elements = document.getElementsByTagName('*');
    for (var i = 0; i < elements.length; i++) {
      var element = elements[i];
      for (var j = 0; j < element.childNodes.length; j++) {
        var node = element.childNodes[j];
        if (node.nodeType === 3) {
          var text = node.nodeValue;
          if text.startsWith("#") {
            console.log("FOUND text with hashtag " + text);
          }
          /*
          var replacedText = text.replace(/[word or phrase to replace here]/gi, '[new word or phrase]');
          if (replacedText !== text) {
            element.replaceChild(document.createTextNode(replacedText), node);
          }
          */
        }
      }
    }
  });
})();


function handleTextNode(textNode) {
    if(textNode.nodeName !== '#text'
        || textNode.parentNode.nodeName === 'SCRIPT' 
        || textNode.parentNode.nodeName === 'STYLE'
    ) {
        //Don't do anything except on text nodes, which are not children 
        //  of <script> or <style>.
        return;
    }
    let origText = textNode.textContent;
    let newHtml=origText.replace(/\$.*/g,'<a href="http://www.cnn.com">' + origText + '</a>');
    //Only change the DOM if we actually made a replacement in the text.
    //Compare the strings, as it should be faster than a second RegExp operation and
    //  lets us use the RegExp in only one place for maintainability.
    if( newHtml !== origText) {
        let newSpan = document.createElement('span');
        newSpan.innerHTML = newHtml;
        textNode.parentNode.replaceChild(newSpan,textNode);
    }
}

//Testing: Walk the DOM of the <body> handling all non-empty text nodes
function processDocument() {
    //Create the TreeWalker
    let treeWalker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT,{
        acceptNode: function(node) { 
            if(node.textContent.length === 0) {
                //Alternately, could filter out the <script> and <style> text nodes here.
                return NodeFilter.FILTER_SKIP; //Skip empty text nodes
            } //else
            return NodeFilter.FILTER_ACCEPT;
        }
    }, false );
    //Make a list of the text nodes prior to modifying the DOM. Once the DOM is 
    //  modified the TreeWalker will become invalid (i.e. the TreeWalker will stop
    //  traversing the DOM after the first modification).
    let nodeList=[];
    while(treeWalker.nextNode()){
        nodeList.push(treeWalker.currentNode);
    }
    //Iterate over all text nodes, calling handleTextNode on each node in the list.
    nodeList.forEach(function(el){
        handleTextNode(el);
    });
} 
document.addEventListener('click',processDocument,false);
