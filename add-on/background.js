/*
On startup, connect to the "browser_remote" app.
*/
let port = chrome.runtime.connectNative("com.jacobweber.browser_remote");

/*
Listen for messages from the app and log them to the console.
*/
port.onMessage.addListener((message) => {
  console.log("Received message from native host", message);
  if (!message.id) {
    return;
  }

  chrome.tabs.query({
    currentWindow: true,
    active: true,
    url: "<all_urls>",
  }, tabs => {
    if (chrome.runtime.lastError) {
      console.error(chrome.runtime.lastError);
      port.postMessage({
        id: message.id,
        result: ""
      });
    } else if (tabs.length === 0) {
      port.postMessage({
        id: message.id,
        result: ""
      });
    } else {
      chrome.tabs.sendMessage(tabs[0].id, message, {}, response => {
        if (chrome.runtime.lastError) {
          console.error(chrome.runtime.lastError);
        } else {
          console.log("Received response from tab", response);
          port.postMessage({
            id: message.id,
            result: response
          });
        }
      });
    }
  });
});

/*
Listen for the native messaging port closing.
*/
port.onDisconnect.addListener((port) => {
  if (port.error) {
    console.log(`Disconnected due to an error: ${port.error.message}`);
  } else {
    // The port closed for an unspecified reason. If this occurred right after
    // calling `chrome.runtime.connectNative()` there may have been a problem
    // starting the the native messaging client in the first place.
    // https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_messaging#troubleshooting
    console.log(`Disconnected`, port);
  }
});
