#!/usr/bin/env python3
"""Embeds live user queries for the Go query service, over HTTP.

This is the query-time half of the asymmetric BGE encoding split
documented in generate_embeddings.py: passages are embedded offline, in
bulk, with no prefix; queries are embedded here, one at a time, at
request time, WITH the BGE instruction prefix. The prefix is applied
inside this service, not by callers - a caller just sends raw query
text and gets back a ready-to-compare vector, without needing to know
BGE-specific encoding conventions.

Runs as a long-lived process (the model loads once, at startup) so the
Go query service can call it internally for the one thing it can't do
itself: run the PyTorch model that produced the passage embeddings.
See docs/design/27-query-embedding-service.md.

Usage:
    python3 embed_service.py [port]   # default port 8091
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from sentence_transformers import SentenceTransformer

MODEL_NAME = "BAAI/bge-small-en-v1.5"
QUERY_PREFIX = "Represent this sentence for searching relevant passages: "
DEFAULT_PORT = 8091

model = None  # loaded once in main(), before serving


class Handler(BaseHTTPRequestHandler):
    def _send_json(self, status, obj):
        body = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        if self.path != "/embed":
            self._send_json(404, {"error": "not found"})
            return

        length = int(self.headers.get("Content-Length", 0))
        try:
            payload = json.loads(self.rfile.read(length))
            text = payload["text"]
            if not isinstance(text, str) or not text:
                raise ValueError("'text' must be a non-empty string")
        except (json.JSONDecodeError, KeyError, ValueError) as e:
            self._send_json(400, {"error": str(e)})
            return

        vector = model.encode(QUERY_PREFIX + text, normalize_embeddings=True)
        self._send_json(200, {"vector": vector.tolist()})

    def log_message(self, format, *args):
        sys.stderr.write("%s - %s\n" % (self.address_string(), format % args))


def main():
    global model
    port = int(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_PORT

    print(f"loading {MODEL_NAME}...", file=sys.stderr)
    model = SentenceTransformer(MODEL_NAME)

    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    print(f"ready, listening on 127.0.0.1:{port}", file=sys.stderr)
    server.serve_forever()


if __name__ == "__main__":
    main()
