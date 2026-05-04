"""Transport abstraction for PulseRPC clients"""

import json
import urllib
import urllib2


class Transport(object):
    """Abstract base class for RPC transports"""

    def request(self, req):
        """Send a JSON-RPC request and return the response"""
        raise NotImplementedError()


class HttpTransport(Transport):
    """HTTP transport implementation using urllib2"""

    def __init__(self, url, timeout=30, headers=None):
        """Initialize HTTP transport"""
        self.url = url
        self.timeout = timeout
        self.headers = headers or {}

    def request(self, req):
        """Send request via HTTP POST"""
        body = json.dumps(req).encode('utf-8')

        headers = {'Content-Type': 'application/json'}
        headers.update(self.headers)

        http_req = urllib2.Request(self.url, data=body, headers=headers)

        try:
            response = urllib2.urlopen(http_req, timeout=self.timeout)
            response_body = response.read().decode('utf-8')

            if not response_body:
                return {}

            return json.loads(response_body)
        except urllib2.HTTPError as e:
            response_body = e.read().decode('utf-8')
            if response_body:
                return json.loads(response_body)
            return {}
        except Exception as e:
            raise ValueError("HTTP request failed: %s" % str(e))


class InProcTransport(Transport):
    """In-process transport for testing (directly calls Server)"""

    def __init__(self, server):
        """Initialize in-process transport"""
        self.server = server

    def request(self, req):
        """Send request directly to Server instance"""
        ctx = req.get('ctx')
        response = self.server.call(req, ctx)
        return response if response is not None else {}
