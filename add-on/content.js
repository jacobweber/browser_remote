chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  console.log("Received message from background script:", message);
  const response = Function(`"use strict";return (${message.query});`)();
  console.log("Sending response to background script:", response);
  // stringify may throw error on circular references
  sendResponse(JSON.parse(JSON.stringify(response)));
  return true;
});
