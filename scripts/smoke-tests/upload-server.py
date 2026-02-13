#!/usr/bin/env python3
"""upload-server.py — Zero-dependency test upload server mimicking Rumble-style uploads.

Endpoints:
  GET  /              — Landing page, sets session cookie
  GET  /upload        — Upload form (requires session cookie, generates CSRF token)
  POST /upload        — Upload receiver (validates cookie, CSRF, file, required fields)
  GET  /upload/success — Confirmation page after successful upload
  GET  /api/last-upload — JSON of last upload for programmatic verification
  GET  /health        — Health check

Usage:
  python3 upload-server.py [PORT]       # default 9876
"""
import hashlib
import http.server
import json
import os
import sys
import time
import urllib.parse
from io import BytesIO


# ── State ──────────────────────────────────────────────────
csrf_tokens = {}  # session -> token
last_upload = {}  # last successful upload details
upload_counter = 0


def parse_multipart(content_type, body):
    """Parse multipart/form-data body. Returns (fields, files) dicts."""
    # Extract boundary from content-type
    parts = content_type.split("boundary=")
    if len(parts) < 2:
        return {}, {}
    boundary = parts[1].strip()
    if boundary.startswith('"') and boundary.endswith('"'):
        boundary = boundary[1:-1]

    fields = {}
    files = {}
    boundary_bytes = ("--" + boundary).encode()
    end_boundary = boundary_bytes + b"--"

    # Split body by boundary
    raw_parts = body.split(boundary_bytes)
    for part in raw_parts:
        if not part or part.strip() == b"" or part.strip() == b"--":
            continue
        if part.startswith(b"--"):
            continue

        # Split headers from body
        header_end = part.find(b"\r\n\r\n")
        if header_end == -1:
            continue
        header_data = part[:header_end].decode("utf-8", errors="replace")
        part_body = part[header_end + 4:]

        # Strip trailing \r\n
        if part_body.endswith(b"\r\n"):
            part_body = part_body[:-2]

        # Parse Content-Disposition
        name = None
        filename = None
        for line in header_data.split("\r\n"):
            if "Content-Disposition" in line:
                for item in line.split(";"):
                    item = item.strip()
                    if item.startswith("name="):
                        name = item.split("=", 1)[1].strip('"')
                    elif item.startswith("filename="):
                        filename = item.split("=", 1)[1].strip('"')

        if name is None:
            continue
        if filename is not None:
            files[name] = {"filename": filename, "data": part_body}
        else:
            fields[name] = part_body.decode("utf-8", errors="replace")

    return fields, files


class UploadHandler(http.server.BaseHTTPRequestHandler):
    """HTTP handler for the test upload server."""

    def log_message(self, format, *args):
        """Suppress default request logging."""
        pass

    def _send_html(self, code, html):
        self.send_response(code)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(html.encode())

    def _send_json(self, code, obj):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(obj).encode())

    def _get_session(self):
        cookie_header = self.headers.get("Cookie", "")
        for pair in cookie_header.split(";"):
            pair = pair.strip()
            if pair.startswith("session="):
                return pair.split("=", 1)[1]
        return None

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)

        if path == "/health":
            self._send_json(200, {"ok": True})
            return

        if path == "/":
            session_id = f"test-session-{int(time.time())}"
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Set-Cookie", f"session={session_id}; Path=/; HttpOnly")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html><head><title>Test Upload Platform</title></head>
<body>
<h1>Test Upload Platform</h1>
<p>Session: {session_id}</p>
<a href="/upload">Go to Upload</a>
</body></html>"""
            self.wfile.write(html.encode())
            return

        if path == "/upload":
            session = self._get_session()
            if not session:
                self._send_html(401, "<h1>401 Not logged in</h1><p>Visit / first to get a session cookie.</p>")
                return

            # Generate CSRF token for this session
            token = hashlib.sha256(f"{session}-{time.time()}".encode()).hexdigest()[:32]
            csrf_tokens[session] = token

            html = f"""<!DOCTYPE html>
<html><head><title>Upload</title></head>
<body>
<h1>Upload File</h1>
<form method="POST" action="/upload" enctype="multipart/form-data">
  <input type="hidden" name="csrf_token" value="{token}">
  <p><label>File: <input type="file" id="file-input" name="Filedata" accept="video/*,image/*,.txt,.pdf"></label></p>
  <p><label>Title: <input type="text" name="title" required></label></p>
  <p><label>Description: <textarea name="description"></textarea></label></p>
  <p><label>Tags: <input type="text" name="tags"></label></p>
  <button type="submit">Upload</button>
</form>
</body></html>"""
            self._send_html(200, html)
            return

        if path == "/upload/success":
            upload_id = query.get("id", ["unknown"])[0]
            info = last_upload if last_upload.get("id") == upload_id else {}
            html = f"""<!DOCTYPE html>
<html><head><title>Upload Success</title></head>
<body>
<h1>Upload Successful</h1>
<p>Upload ID: {upload_id}</p>
<p>Filename: {info.get('name', 'N/A')}</p>
<p>Size: {info.get('size', 'N/A')} bytes</p>
<p>MD5: {info.get('md5', 'N/A')}</p>
<p>Title: {info.get('title', 'N/A')}</p>
</body></html>"""
            self._send_html(200, html)
            return

        if path == "/api/last-upload":
            self._send_json(200, last_upload if last_upload else {"error": "no uploads yet"})
            return

        self._send_html(404, "<h1>404 Not Found</h1>")

    def do_POST(self):
        global last_upload, upload_counter

        parsed = urllib.parse.urlparse(self.path)
        if parsed.path != "/upload":
            self._send_html(404, "<h1>404 Not Found</h1>")
            return

        # Check session cookie
        session = self._get_session()
        cookie_ok = session is not None
        if not cookie_ok:
            self._send_html(401, "<h1>401 Not logged in</h1><p>Session cookie required.</p>")
            return

        # Read body
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        content_type = self.headers.get("Content-Type", "")

        # Parse multipart
        fields, files = parse_multipart(content_type, body)

        # Check CSRF
        csrf_sent = fields.get("csrf_token", "")
        csrf_expected = csrf_tokens.get(session, "")
        csrf_ok = csrf_sent != "" and csrf_sent == csrf_expected
        if not csrf_ok:
            self._send_html(403, "<h1>403 CSRF token expired</h1><p>CSRF token mismatch.</p>")
            return

        # Check file
        file_entry = files.get("Filedata")
        if not file_entry:
            self._send_html(422, "<h1>422 No file uploaded</h1><p>The Filedata field is required.</p>")
            return
        if len(file_entry["data"]) == 0:
            self._send_html(422, "<h1>422 Empty file</h1><p>File must not be empty.</p>")
            return

        # Check required field: title
        title = fields.get("title", "")
        if not title:
            self._send_html(422, "<h1>422 Missing title</h1><p>The title field is required.</p>")
            return

        # Success
        upload_counter += 1
        upload_id = f"upload-{upload_counter}"
        file_data = file_entry["data"]
        md5 = hashlib.md5(file_data).hexdigest()

        last_upload = {
            "id": upload_id,
            "name": file_entry["filename"],
            "size": len(file_data),
            "md5": md5,
            "title": title,
            "tags": fields.get("tags", ""),
            "csrf_ok": csrf_ok,
            "cookie_ok": cookie_ok,
        }

        # 302 redirect to success page
        self.send_response(302)
        self.send_header("Location", f"/upload/success?id={upload_id}")
        self.end_headers()


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9876
    server = http.server.HTTPServer(("127.0.0.1", port), UploadHandler)
    print(f"Upload test server on http://127.0.0.1:{port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


if __name__ == "__main__":
    main()
