browser.runtime.onMessage.addListener(function (message) {
  console.log("Received message from background script:", message);
  const response = Function(`"use strict";return (${message.query});`)();
  console.log("Sending response to background script:", response);
  return Promise.resolve(JSON.stringify(response));
});
