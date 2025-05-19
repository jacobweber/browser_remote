chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  console.log("Received message from background script:", message);
  const response = Function(`"use strict";return (${message.query});`)();
  console.log("Sending response to background script:", response);
  sendResponse(JSON.stringify(response));
  return true;
});
