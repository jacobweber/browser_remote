window.addEventListener("load", () => {
  // Request status from background script, and display it.
  chrome.runtime.sendMessage(null, "status", null, (response) => {
    if (response === null) {
      document.getElementById("error").style.display = 'block';
      document.getElementById("success").style.display = 'none';
    } else {
      document.getElementById("address").innerText = response.address;
      document.getElementById("address2").innerText = response.address;
      document.getElementById("error").style.display = 'none';
      document.getElementById("success").style.display = 'block';
    }
  });
});
