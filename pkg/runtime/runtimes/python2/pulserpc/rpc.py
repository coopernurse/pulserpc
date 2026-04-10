"""RPC error handling for JSON-RPC 2.0"""

try:
    unicode
except NameError:
    unicode = str


class RPCError(Exception):
    """Exception class for JSON-RPC errors"""
    
    def __init__(self, code, message, data=None):
        self.code = code
        self.message = message
        self.data = data
        super(RPCError, self).__init__("RPCError %d: %s" % (code, message))
