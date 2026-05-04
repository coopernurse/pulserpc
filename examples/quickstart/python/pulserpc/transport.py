"""Transport abstraction for PulseRPC clients"""

from abc import ABC, abstractmethod
from typing import Any, Dict
import json
import urllib.request
import urllib.error


class Transport(ABC):
    """Abstract base class for RPC transports"""

    @abstractmethod
    def request(self, req: Dict[str, Any]) -> Dict[str, Any]:
        """Send a JSON-RPC request and return the response

        Args:
            req: JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'

        Returns:
            JSON-RPC response dict

        Raises:
            urllib.error.URLError: For HTTP transport errors
            ValueError: For invalid responses
        """
        pass


class HttpTransport(Transport):
    """HTTP transport implementation using urllib"""

    def __init__(self, url: str, timeout: int = 30, headers: Dict[str, str] = None):
        """Initialize HTTP transport

        Args:
            url: RPC endpoint URL
            timeout: Request timeout in seconds (default: 30)
            headers: Optional HTTP headers to include with requests
        """
        self.url = url
        self.timeout = timeout
        self.headers = headers or {}

    def request(self, req: Dict[str, Any]) -> Dict[str, Any]:
        """Send request via HTTP POST

        Args:
            req: JSON-RPC request dict

        Returns:
            JSON-RPC response dict

        Raises:
            urllib.error.URLError: For network/HTTP errors
            ValueError: For invalid JSON responses
        """
        # Prepare request body
        body = json.dumps(req).encode('utf-8')

        # Create request
        http_req = urllib.request.Request(
            self.url,
            data=body,
            method='POST',
            headers={
                'Content-Type': 'application/json',
                **self.headers
            }
        )

        # Send request and get response
        with urllib.request.urlopen(http_req, timeout=self.timeout) as response:
            response_body = response.read().decode('utf-8')

            # Handle empty response (notification)
            if not response_body:
                return {}

            # Parse JSON response
            try:
                return json.loads(response_body)
            except json.JSONDecodeError as e:
                raise ValueError(f"Invalid JSON response: {e}") from e


class InProcTransport(Transport):
    """In-process transport for testing (directly calls Server)"""

    def __init__(self, server):
        """Initialize in-process transport

        Args:
            server: Server instance to call directly
        """
        self.server = server

    def request(self, req: Dict[str, Any]) -> Dict[str, Any]:
        """Send request directly to Server instance

        Args:
            req: JSON-RPC request dict

        Returns:
            JSON-RPC response dict (or empty dict for notifications)
        """
        ctx = req.get('ctx')
        response = self.server.call(req, ctx)
        return response if response is not None else {}
