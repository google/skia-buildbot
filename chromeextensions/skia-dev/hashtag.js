// Copyright (c) 2019 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function processTextNode(textNode) {
  let origText = textNode.textContent;
  // Replace all hashtags with links to hashtag.skia.org.
  let newText = origText.replace(/(^|\s)#(\S+)/g,
                                 '$1<a href="https://hashtag.skia.org/?hashtag=$2">#$2</a>');
  if( newText !== origText) {
      console.debug(`hashtag.js changing '${origText}' to '${newText}'`);
      const newSpan = document.createElement('span');
      newSpan.innerHTML = newText;
      textNode.parentNode.replaceChild(newSpan, textNode);
  }
}

function processHTML() {
  // Create a TreeWalker to accept non-empty text nodes that are not children of
  // <script> or <style>.
  const treeWalker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
    acceptNode: (node) => {
      if (node.nodeName !== '#text'
            || node.parentNode.nodeName === 'SCRIPT'
            || node.parentNode.nodeName === 'STYLE') {
        // Skip text nodes that are not children of <script> or <style>.
        return NodeFilter.FILTER_SKIP;
      } else if(node.textContent.length === 0) {
        // Skip empty text nodes.
        return NodeFilter.FILTER_SKIP;
      } else {
        return NodeFilter.FILTER_ACCEPT;
      }
    }
  }, false );

  // Gather all text nodes to process. Do not modify them yet or the treeWalker
  // will become invalid.
  const nodeList = [];
  while (treeWalker.nextNode()) {
    nodeList.push(treeWalker.currentNode);
  }
  // Now iterate over all text nodes looking for hashtags.
  nodeList.forEach((el) => processTextNode(el));
}

processHTML();
