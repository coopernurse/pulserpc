package com.bitmechanic.pulserpc;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

public class Client {
    private final Transport transport;
    private final boolean validateRequest;
    private final boolean validateResponse;
    private int requestID = 0;
    private final Map<String, InterfaceClientProxy> ifaces = new HashMap<>();
    private ContractAuditor auditor;
    private boolean verifyOnBootstrap;
    private Object localIDL;

    public Contract contract;

    public Client(Transport transport, boolean validateRequest, boolean validateResponse) {
        this.transport = transport;
        this.validateRequest = validateRequest;
        this.validateResponse = validateResponse;

        bootstrap();
    }

    private Client(Transport transport, boolean validateRequest, boolean validateResponse, ContractAuditor auditor, boolean verifyOnBootstrap) {
        this.transport = transport;
        this.validateRequest = validateRequest;
        this.validateResponse = validateResponse;
        this.auditor = auditor;
        this.verifyOnBootstrap = verifyOnBootstrap;

        bootstrap();

        if (this.verifyOnBootstrap) {
            verifyCompatibility();
        }
    }

    public static Client create(Transport transport, boolean validateRequest, boolean validateResponse) {
        return new Client(transport, validateRequest, validateResponse);
    }

    public static Client create(Transport transport, boolean validateRequest, boolean validateResponse, ContractAuditor auditor, boolean verifyOnBootstrap) {
        return new Client(transport, validateRequest, validateResponse, auditor, verifyOnBootstrap);
    }

    public Client withAuditor(ContractAuditor auditor) {
        this.auditor = auditor;
        return this;
    }

    public Client verifyOnBootstrap() {
        this.verifyOnBootstrap = true;
        return this;
    }

    public InterfaceClientProxy getInterface(String ifaceName) {
        return ifaces.get(ifaceName);
    }

    public Object call(String method, Object params) {
        if (validateRequest && contract != null) {
            String[] parts = parseMethodName(method);
            if (parts != null) {
                List<Object> paramList = convertParamsToList(parts[0], parts[1], params);
                if (paramList != null) {
                    try {
                        contract.validateRequest(parts[0], parts[1], paramList);
                    } catch (IllegalArgumentException ex) {
                        throw new RuntimeException("Request validation failed: " + ex.getMessage(), ex);
                    }
                }
            }
        }

        requestID++;
        Object reqId = requestID;

        Request request = new Request();
        request.setJsonrpc("2.0");
        request.setMethod(method);
        request.setId(reqId);
        request.setParams(params);

        Response response;
        try {
            response = transport.call(request);
        } catch (Exception e) {
            throw new RuntimeException("Transport request failed: " + e.getMessage(), e);
        }

        if (response.hasError()) {
            Map<String, Object> errObj = response.getError();
            int code = errObj.containsKey("code") ? ((Number) errObj.get("code")).intValue() : -32603;
            String message = errObj.containsKey("message") ? (String) errObj.get("message") : "Unknown error";
            Object data = errObj.get("data");
            throw new RPCError(code, message, data);
        }

        Object result = response.getResult();

        if (validateResponse && contract != null && result != null) {
            String[] parts = parseMethodName(method);
            if (parts != null) {
                try {
                    contract.validateResponse(parts[0], parts[1], result);
                } catch (IllegalArgumentException ex) {
                    throw new RuntimeException("Response validation failed: " + ex.getMessage(), ex);
                }
            }
        }

        return result;
    }

    public void notify(String method, Object params) {
        try {
            call(method, params);
        } catch (Exception e) {
        }
    }

    private void bootstrap() {
        Request req = new Request("pulserpc-idl", null, "bootstrap");

        Response resp;
        try {
            resp = transport.call(req);
        } catch (Exception e) {
            throw new RuntimeException("Failed to fetch IDL from server: " + e.getMessage(), e);
        }

        if (resp.hasError()) {
            throw new RuntimeException("Failed to fetch IDL from server: " + resp.getError().get("message"));
        }

        Object idlJson = resp.getResult();
        if (idlJson == null) {
            throw new RuntimeException("Server returned empty IDL");
        }

        contract = new Contract(idlJson);

        for (Map.Entry<String, Interface> entry : contract.interfaces.entrySet()) {
            InterfaceClientProxy proxy = new InterfaceClientProxy(this, entry.getValue());
            ifaces.put(entry.getKey(), proxy);
        }
    }

    private String[] parseMethodName(String method) {
        int lastDot = method.lastIndexOf('.');
        if (lastDot < 0) {
            return null;
        }
        return new String[]{method.substring(0, lastDot), method.substring(lastDot + 1)};
    }

    @SuppressWarnings("unchecked")
    private List<Object> convertParamsToList(String ifaceName, String funcName, Object params) {
        if (contract == null) return null;

        Interface iface = contract.getInterface(ifaceName);
        if (iface == null) return null;

        FunctionDef fn = iface.getFunction(funcName);
        if (fn == null) return null;

        List<Object> paramDefs = fn.containsKey("parameters") && fn.get("parameters") instanceof List
            ? (List<Object>) fn.get("parameters")
            : new java.util.ArrayList<>();

        List<Object> result = new java.util.ArrayList<>();

        if (params instanceof List) {
            result.addAll((List<Object>) params);
        } else if (params instanceof Map) {
            Map<String, Object> namedParams = (Map<String, Object>) params;
            for (Object paramDef : paramDefs) {
                if (paramDef instanceof Map) {
                    Map<String, Object> pd = (Map<String, Object>) paramDef;
                    String name = pd.containsKey("name") ? (String) pd.get("name") : "";
                    result.add(namedParams.get(name));
                } else {
                    result.add(null);
                }
            }
        } else if (params != null) {
            result.add(params);
        }

        return result;
    }

    public VerificationResult verifyCompatibility() {
        Object clientIDL;
        if (localIDL != null) {
            clientIDL = localIDL;
        } else if (contract != null) {
            clientIDL = contract.getIdlParsed();
        } else {
            clientIDL = null;
        }

        Object serverIDL = contract != null ? contract.getIdlParsed() : null;

        List<ContractDelta> deltas = new java.util.ArrayList<>();
        if (clientIDL != null && serverIDL != null) {
            deltas = DiffEngine.diffIDL(clientIDL, serverIDL);
        }

        boolean compatible = true;
        for (ContractDelta delta : deltas) {
            if (delta.getSeverity() == ContractDelta.Severity.Error) {
                compatible = false;
                break;
            }
        }

        VerificationResult result = new VerificationResult(
            compatible,
            serverIDL != null ? DiffEngine.computeChecksum(serverIDL) : "",
            clientIDL != null ? DiffEngine.computeChecksum(clientIDL) : "",
            deltas,
            System.currentTimeMillis()
        );

        if (auditor != null) {
            auditor.audit(result);
        }

        return result;
    }

    public Client setLocalIDL(String idlJson) {
        JsonParser parser = new JacksonJsonParser();
        this.localIDL = parser.fromJson(idlJson, Object.class);
        return this;
    }
}