// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

// Doing a conditional write allows selecting, copying and paste to work
// instead of the selection constantly going away.
function overwriteIfDifferent(elem, str) {
  if (elem.innerHTML !== str) {
    elem.innerHTML = str;
  }
}

function drawExtraTableEntries(jobID, gerritIssue, gerritPatchSet, sourceRepo, sourceRevision, retryOf, parentTaskId, thisTaskId) {
  var elem = document.getElementById("extra_task_scheduler");
  if (jobID) {
    overwriteIfDifferent(elem, `<td>Forced Job ID</td><td colspan="2"><a rel="noopener" target="_blank" href="https://task-scheduler.skia.org/job/20161205T172828.520126401Z_000000000000d128">${jobID}</a></td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_gerrit");
  if (gerritIssue && gerritPatchSet) {
    overwriteIfDifferent(elem, `<td>Associated CL</td><td colspan="2"><a rel="noopener" target="_blank" href="https://skia-review.googlesource.com/c/${gerritIssue}/${gerritPatchSet}">review.skia.org/${gerritIssue}</a></td>`);
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
    overwriteIfDifferent(elem, `<td>${key}</td><td colspan="2"><a rel="noopener" target="_blank" href="${link}">${shortRevision}</a></td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_retry_of");
  if (retryOf) {
    overwriteIfDifferent(elem, `<td>Retry of Task</td><td colspan="2"><a rel="noopener" target="_blank" href="https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_id%3A${retryOf}&amp;l=10&amp;s=created_ts%3Adesc">${retryOf}</a></td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_depends_on");
  if (parentTaskId) {
    overwriteIfDifferent(elem, `<td>Parent Task</td><td colspan="2"><a rel="noopener" target="_blank" href="https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_id%3A${parentTaskId}&amp;l=10&amp;s=created_ts%3Adesc">${parentTaskId}</a></td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_dependents");
  if (thisTaskId) {
    overwriteIfDifferent(elem, `<td>Child Tasks</td><td colspan="2"><a rel="noopener" target="_blank" href="https://chromium-swarm.appspot.com/tasklist?c=name&amp;c=state&amp;c=created_ts&amp;c=user&amp;f=sk_parent_task_id%3A${thisTaskId}&amp;l=10&amp;s=created_ts%3Adesc">Find in Task List</a></td>`);
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
