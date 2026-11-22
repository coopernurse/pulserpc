"""Transport abstraction for PulseRPC clients"""

import json

try:
    from urllib.request import Request, urlopen
    from urllib.error import HTTPError
except ImportError:
    _u2 = __import__("urllib2")
    Request = _u2.Request
    urlopen = _u2.urlopen
    HTTPError = _u2.HTTPError

# Defaults shared by every HTTP client (D5/D6, R17/R19).
DEFAULT_TIMEOUT = 30
MAX_RESPONSE_BODY_SIZE = 10 * 1024 * 1024  # 10 MiB


def _parse_response(response_body):
    """Decode a JSON response body, normalizing parse failures.

    Python 2's ``json`` module has no ``JSONDecodeError``; every parse failure
    (and a decode failure) is a plain ``ValueError``.  Wrapping it here keeps
    the error identifiable as an invalid response while leaving the distinct
    "response body exceeds maximum size" ValueError raised by
    ``HttpTransport.request`` untouched.
    """
    try:
        return json.loads(response_body)
    except ValueError as e:
        raise ValueError("Invalid JSON response: %s" % e)


class Transport(object):
    """Abstract base class for RPC transports"""

    def request(self, req):
        """Send a JSON-RPC request and return the response"""
        raise NotImplementedError()


class HttpTransport(Transport):
    """HTTP transport implementation using urllib2"""

    def __init__(self, url, timeout=DEFAULT_TIMEOUT, headers=None,
                 max_response_size=MAX_RESPONSE_BODY_SIZE):
        """Initialize HTTP transport

        Every trailing ``/`` is stripped from ``url`` via ``url.rstrip("/")`` (R30).
        ``headers`` are merged over the default Content-Type (R29).
        """
        self.url = url.rstrip("/") if url else url
        self.timeout = timeout
        self.headers = headers or {}
        self.max_response_size = max_response_size

    def request(self, req):
        """Send request via HTTP POST"""
        body = json.dumps(req).encode("utf-8")

        headers = {"Content-Type": "application/json"}
        headers.update(self.headers)

        http_req = Request(self.url, data=body, headers=headers)

        try:
            response = urlopen(http_req, timeout=self.timeout)
            # Read one byte past the cap so oversized responses are rejected
            # without buffering the entire body (R17).
            raw_body = response.read(self.max_response_size + 1)
            if len(raw_body) > self.max_response_size:
                raise ValueError(
                    "Response body exceeds maximum size of %d bytes"
                    % self.max_response_size
                )
            response_body = raw_body.decode("utf-8")

            if not response_body:
                return {}

            return _parse_response(response_body)
        except HTTPError as e:
            raw_body = e.read(self.max_response_size + 1)
            if len(raw_body) > self.max_response_size:
                raise ValueError(
                    "Response body exceeds maximum size of %d bytes"
                    % self.max_response_size
                )
            response_body = raw_body.decode("utf-8")
            if response_body:
                return _parse_response(response_body)
            return {}
        except ValueError:
            raise
        except Exception as e:
            raise ValueError("HTTP request failed: %s" % str(e))


class InProcTransport(Transport):
    """In-process transport for testing (directly calls Server)"""

    def __init__(self, server):
        """Initialize in-process transport"""
        self.server = server

    def request(self, req):
        """Send request directly to Server instance"""
        ctx = req.get("ctx")
        response = self.server.call(req, ctx)
        return response if response is not None else {}
