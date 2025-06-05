## Browser Remote

Allows you to evaluate and execute JavaScript expressions in a browser, by making a request to a local web server. Useful for automation.

### Installation

Install the `browser_remote` application anywhere on your machine, e.g. in `/path/to/browser_remote`.

Follow the instructions [here](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging#native-messaging-host) to create a native messaging host manifest file for your browser(s).

Example of this file for Chrome:
```
{
  "name": "com.jacobweber.browser_remote",
  "description": "Run JavaScript from other applications",
  "path": "/path/to/browser_remote",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://jgmdchjaeklnmaikghgeiodkegiiedge/"]
}
```
and for Firefox:
```
{
  "name": "com.jacobweber.browser_remote",
  "description": "Run JavaScript from other applications",
  "path": "/path/to/browser_remote",
  "type": "stdio",
  "allowed_extensions": ["browser_remote@jacobweber.com"]
}
```

Install the `browser_remote` extension in your browser:

* For Chrome, go to Window > Extensions, enable "Developer mode", click "Load unpacked", and open the extension's folder.

* For Firefox, go to `about:debugging`, click "This Firefox", click "Load Temporary Add-on", and open the extension's `manifest.json`.


Once the extension is loaded, it will start a web server on port 5555, or the next free port. You can see the URL by clicking the extension's icon.

You can now send requests to the web server, and they'll be evaluated in the front browser tab, and returned.

Request format:
```
POST /
{
	// an expression to evaluate and return:
	"query": "location.href"
	// or a function call:
	"query": "window.open(\"https://www.apple.com\")"
	// optional:
	"tabs": "front" (default) | "all"
}
```

Response format:
```
{
	"status": "ok" | "error message"
	// one per tab
	"results": [
		"https://www.google.com"
	]
}
```
