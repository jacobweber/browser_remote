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

  chrome.tabs.query({
    ...query,
    url: ["https://*/*", "http://*/*"],
  }, tabs => {
    if (chrome.runtime.lastError) {
      console.error(chrome.runtime.lastError.message);
      postError(chrome.runtime.lastError.message);
    } else if (tabs.length === 0) {
      postError("no open tabs");
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
