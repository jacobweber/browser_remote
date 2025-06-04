window.addEventListener("load", () => {
  chrome.runtime.sendMessage(null, "status", null, (response) => {
    const addr = document.getElementById("address");
    addr.innerText = response.address;
  });
});
