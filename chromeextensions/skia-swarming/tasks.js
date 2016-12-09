// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

function addTableEntry(elem, title, linkHref, linkText) {
  if (elem.children.length != 2){
    elem.innerHTML = "";
    var newCol = document.createElement("td");
    elem.appendChild(newCol);
    newCol = document.createElement("td");
    var newLink = document.createElement("a");
    newCol.appendChild(newLink);
    elem.appendChild(newCol);
  }

  var first = elem.children[0];
  var link = elem.children[1].children[0];

  first.innerText = title;

  link.href = linkHref;
  link.innerText = linkText;
  link.rel = "noopener";
  link.target = "_blank";
}

function drawExtraTableEntries(jobID, gerritIssue, gerritPatchSet, sourceRepo, sourceRevision, retryOf, parentTaskId, thisTaskId) {
  var elem = document.getElementById("extra_task_scheduler");
  if (jobID) {
    var link = "https://task-scheduler.skia.org/job/" + jobID;
    addTableEntry(elem, "Forced Job ID", link, jobID);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_gerrit");
  if (gerritIssue && gerritPatchSet) {
    var link = `https://skia-review.googlesource.com/c/${gerritIssue}/${gerritPatchSet}`;
    addTableEntry(elem, "Associated CL", link, "review.skia.org/" + gerritIssue);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_commit");
  if (sourceRepo && sourceRevision) {
    var key = "Associated Commit";
    if (gerritIssue && gerritPatchSet) {
      key = "Commit Patchset Was Applied To";
    }
    var link = sourceRepo.replace("%s", sourceRevision);
    var shortRevision = sourceRevision.substring(0, 12);
    addTableEntry(elem, key, link, shortRevision);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_retry_of");
  if (retryOf) {
    var link = `https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_id%3A${retryOf}&amp;l=10&amp;s=created_ts%3Adesc`;
    addTableEntry(elem, "Retry of Task", link, retryOf);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_depends_on");
  if (parentTaskId) {
    var link = `https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_id%3A${parentTaskId}&amp;l=10&amp;s=created_ts%3Adesc`;
    addTableEntry(elem, "Parent Task", link, parentTaskId);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_dependents");
  if (thisTaskId) {
    var link = `https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_parent_task_id%3A${thisTaskId}&amp;l=10&amp;s=created_ts%3Adesc`;
    addTableEntry(elem, "Child Tasks", link, "Find in Task List");
  } else {
    elem.innerHTML = "";
  }
}

function createExtraRow(parentNode, newID) {
  var newRow = document.createElement("tr");
  newRow.className = "outline";
  newRow.id = newID;
  parentNode.appendChild(newRow);
}

window.addEventListener("WebComponentsReady", function(e) {
  // Create our extra <tr> elements.
  var moreDetails = document.getElementById("more_details");
  createExtraRow(moreDetails.parentNode, "extra_task_scheduler");
  createExtraRow(moreDetails.parentNode, "extra_gerrit");
  createExtraRow(moreDetails.parentNode, "extra_commit");
  createExtraRow(moreDetails.parentNode, "extra_retry_of");
  createExtraRow(moreDetails.parentNode, "extra_depends_on");
  createExtraRow(moreDetails.parentNode, "extra_dependents");

  window.setInterval(function(){
    var id = document.getElementById("input").value;

    if (!id) {
      return;
    }

    // a map of all task tags that begins with sk_ or source_
    var skData = {};

    // The tbody #more_details has all of the tags in it, which are in the form
    // tag:value.  We iterate through all of the tds looking for those that
    // are needed to render our extra <tr> elements.
    var cells = moreDetails.getElementsByTagName("td");
    for (var i = 0; i < cells.length; i++) {
      var content = cells[i].textContent.trim();
      if (content.startsWith("sk_") || content.startsWith("source_")) {
        var split = content.split(":", 1);
        var tag = split[0];
        var rest = content.substring(tag.length + 1);
        skData[tag] = rest;
      }
    }

    drawExtraTableEntries(
        skData.sk_forced_job_id,
        skData.sk_issue,
        skData.sk_patchset,
        skData.source_repo,
        skData.source_revision,
        skData.sk_retry_of,
        skData.sk_parent_task_id,
        skData.sk_id);
  }, 100);

});
})();
