package com.bitmechanic.pulserpc;

import java.util.List;

public class VerificationResult {
    private final boolean compatible;
    private final String serverChecksum;
    private final String clientChecksum;
    private final List<ContractDelta> deltas;
    private final long timestamp;

    public VerificationResult(boolean compatible, String serverChecksum, String clientChecksum,
                              List<ContractDelta> deltas, long timestamp) {
        this.compatible = compatible;
        this.serverChecksum = serverChecksum;
        this.clientChecksum = clientChecksum;
        this.deltas = deltas;
        this.timestamp = timestamp;
    }

    public boolean isCompatible() { return compatible; }
    public String getServerChecksum() { return serverChecksum; }
    public String getClientChecksum() { return clientChecksum; }
    public List<ContractDelta> getDeltas() { return deltas; }
    public long getTimestamp() { return timestamp; }
}