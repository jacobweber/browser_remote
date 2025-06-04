// Evaluate message in tab context, and send back result.
chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  console.log("Received message from background script:", message);
  try {
    const response = Function(`"use strict";return (${message.query});`)();
    console.log("Sending response to background script:", response);
    // stringify may throw error on circular references
    sendResponse({ status: "ok", result: JSON.parse(JSON.stringify(response)) });
  } catch (err) {
    sendResponse({ status: err.toString(), result: null });
  }
  return true;
});
