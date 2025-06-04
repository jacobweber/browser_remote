// On startup, connect to the native app.
let port = chrome.runtime.connectNative("com.jacobweber.browser_remote");

let nativeStatus = null;

// Listen for messages from content scripts.
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  // Popup will request status which we previously received from native app
  if (message === "status") {
    sendResponse(nativeStatus);
  }
});

// Listen for messages from native app.
port.onMessage.addListener((message) => {
  console.log("Received message from native app", message);
  if (!message.id) {
    return;
  }

  // Native app will send status immediately upon launch; store for when popup requests it
  if (message.id === 'status') {
    nativeStatus = message.result;
    return;
  }

  const postError = status => {
    port.postMessage({
      id: message.id,
      status,
      results: []
    });
  }

  let query = {}
  if (message.tabs === 'all') {
    query = {}
  } else { // front
    query = {
      currentWindow: true,
      active: true,
    }
  }

  // Send message to tabs, wait for their responses, and return combined response to native app.
  chrome.tabs.query({
    ...query,
    url: ["https://*/*", "http://*/*"],
  }, tabs => {
    if (chrome.runtime.lastError) {
      console.error(chrome.runtime.lastError.message);
      postError(chrome.runtime.lastError.message);
    } else if (tabs.length === 0) {
      postError("no tabs found");
    } else {
      Promise.all(tabs.map(tab => new Promise((resolve, reject) => {
        chrome.tabs.sendMessage(tab.id, message, {}, response => {
          if (chrome.runtime.lastError) {
            reject(chrome.runtime.lastError.message);
          } else {
            console.log("Received response from tab", response);
            if (response.status !== "ok") {
              reject(response.status);
            } else {
              resolve(response.result);
            }
          }
        });
      }))).then(results => {
        port.postMessage({
          id: message.id,
          status: "ok",
          results,
        });
      }).catch(error => {
        console.error(error);
        postError(error);
      });
    }
  });
});

// Listen for the native messaging port closing.
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
