#!/usr/bin/env python3

import socket
import time
import syslog
import sys
import json
import struct
import http.server
import threading

http_hostname = "localhost"
http_port = 5555
http_check_interval = 0.5

def get_message_from_browser():
    raw_length = sys.stdin.buffer.read(4)
    if len(raw_length) == 0:
        sys.exit(0)
    message_length = struct.unpack('@I', raw_length)[0]
    message = sys.stdin.buffer.read(message_length).decode('utf-8')
    return json.loads(message)

def encode_message_for_browser(message_content):
    encoded_content = json.dumps(message_content, separators=(',', ':')).encode('utf-8')
    encoded_length = struct.pack('@I', len(encoded_content))
    return {'length': encoded_length, 'content': encoded_content}

def send_message_to_browser(encoded_message):
    sys.stdout.buffer.write(encoded_message['length'])
    sys.stdout.buffer.write(encoded_message['content'])
    sys.stdout.buffer.flush()

def is_port_in_use(port, host):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex((host, port)) == 0

def wait_for_port_free(port, host, check_interval):
    tries = 0
    max_tries = 10
    while is_port_in_use(port, host) and tries < max_tries:
        time.sleep(check_interval)
        tries += 1
    if tries == max_tries:
        raise Exception(f"Port {port} on {host} is still in use after {max_tries} tries.")

handlers: dict = {}

"""
Request format:
POST /
{
    // an expression to evaluate and return:
    "expr": "location.href"
    // or a function call:
    "expr": "window.open(\"https://www.apple.com\")"
}

Response format:
{
    "status": "ok" | "error"
    "result": "https://www.google.com"
}
"""
class SimpleHTTPRequestHandler(http.server.BaseHTTPRequestHandler):
    def get_post_request(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        if not post_data:
            raise Exception("Invalid POST data")
        message = json.loads(post_data.decode('utf-8'))
        if not message or not isinstance(message, dict):
            raise Exception("Invalid POST data")
        return message

    def do_POST(self):
        try:
            request = self.get_post_request();
            if "expr" in request:
                send_message_to_browser(encode_message_for_browser(request["expr"]))

            response = json.dumps({"status": "ok"}).encode('utf-8')
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)
        except:
            response = json.dumps({"status": "error"}).encode('utf-8')
            self.send_response(400)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

def serve_http():
    try:
        wait_for_port_free(http_port, http_hostname, http_check_interval)
        httpd = http.server.HTTPServer((http_hostname, http_port), SimpleHTTPRequestHandler)
        thread = threading.Thread(target=httpd.serve_forever(), args=(httpd, ))
        thread.start()
        syslog.syslog(syslog.LOG_INFO, f"HTTP server started on {http_hostname}:{http_port}")
    except Exception as e:
        syslog.syslog(syslog.LOG_ERR, f"Error starting HTTP server: {e}")
        sys.exit(1)

serve_http()
while True:
    received_message = get_message_from_browser()
    if received_message == "ping":
        send_message_to_browser(encode_message_for_browser("pongx"))
