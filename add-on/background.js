/*
On startup, connect to the "browser_remote" app.
*/
let port = browser.runtime.connectNative("browser_remote");

const sendMessageToTab = (tab, message) => {
  browser.tabs
    .sendMessage(tab.id, message)
    .then(response => {
      console.log("Response from the content script:", response);
    })
    .catch(console.error);
}

/*
Listen for messages from the app and log them to the console.
*/
port.onMessage.addListener((message) => {
  console.log("Received message from native host", message);
  if (!message.id) {
    return;
  }

  browser.tabs.query({
    currentWindow: true,
    active: true,
  })
  .then(tabs => {
    for (const tab of tabs) {
      browser.tabs.sendMessage(tab.id, message)
      .then(response => {
        console.log("Received response from tab", response);
        port.postMessage({
          id: message.id,
          result: response
        });
      })
      .catch(console.error);
    }
  })
  .catch(console.error);
});

/*
Listen for the native messaging port closing.
*/
port.onDisconnect.addListener((port) => {
  if (port.error) {
    console.log(`Disconnected due to an error: ${port.error.message}`);
  } else {
    // The port closed for an unspecified reason. If this occurred right after
    // calling `browser.runtime.connectNative()` there may have been a problem
    // starting the the native messaging client in the first place.
    // https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_messaging#troubleshooting
    console.log(`Disconnected`, port);
  }
});
