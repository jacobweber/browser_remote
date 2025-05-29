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

  const postError = status => {
    port.postMessage({
      id: message.id,
      status,
      result: null
    });
  }

  chrome.tabs.query({
    currentWindow: true,
    active: true,
    url: ["https://*/*", "http://*/*"],
  }, tabs => {
    if (chrome.runtime.lastError) {
      console.error(chrome.runtime.lastError.message);
      postError(chrome.runtime.lastError.message);
    } else if (tabs.length === 0) {
      postError("no open windows");
    } else {
      chrome.tabs.sendMessage(tabs[0].id, message, {}, response => {
        if (chrome.runtime.lastError) {
          console.error(chrome.runtime.lastError.message);
          postError(chrome.runtime.lastError.message);
        } else {
          console.log("Received response from tab", response);
          port.postMessage({
            id: message.id,
            status: response.status,
            result: response.result
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
