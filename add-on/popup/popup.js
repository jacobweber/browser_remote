window.addEventListener("load", () => {
  // Request status from background script, and display it.
  chrome.runtime.sendMessage(null, "status", null, (response) => {
    const addr = document.getElementById("address");
    addr.innerText = response.address;
  });
});
