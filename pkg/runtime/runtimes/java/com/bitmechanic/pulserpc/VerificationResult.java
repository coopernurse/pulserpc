package com.bitmechanic.pulserpc;

import java.util.List;

public class VerificationResult {
    private final boolean compatible;
    private final String error;
    private final String serverChecksum;
    private final String clientChecksum;
    private final List<ContractDelta> deltas;
    private final long timestamp;

    public VerificationResult(boolean compatible, String serverChecksum, String clientChecksum,
                              List<ContractDelta> deltas, long timestamp) {
        this(compatible, serverChecksum, clientChecksum, deltas, timestamp, null);
    }

    public VerificationResult(boolean compatible, String serverChecksum, String clientChecksum,
                              List<ContractDelta> deltas, long timestamp, String error) {
        this.compatible = compatible;
        this.serverChecksum = serverChecksum;
        this.clientChecksum = clientChecksum;
        this.deltas = deltas;
        this.timestamp = timestamp;
        this.error = error;
    }

    public boolean isCompatible() { return compatible; }
    /** Non-null/non-empty when verification could not be completed (inconclusive). */
    public String getError() { return error; }
    public String getServerChecksum() { return serverChecksum; }
    public String getClientChecksum() { return clientChecksum; }
    public List<ContractDelta> getDeltas() { return deltas; }
    public long getTimestamp() { return timestamp; }
}