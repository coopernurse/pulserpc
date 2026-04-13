package com.bitmechanic.pulserpc;

public interface ContractAuditor {
    void audit(VerificationResult result);
    String name();

    static ContractAuditor noOp() {
        return new ContractAuditor() {
            @Override
            public void audit(VerificationResult result) {
            }

            @Override
            public String name() {
                return "NoOp";
            }
        };
    }

    static ContractAuditor logging() {
        return new ContractAuditor() {
            @Override
            public void audit(VerificationResult result) {
                if (!result.isCompatible()) {
                    System.err.println("[ERROR] Contract incompatibility detected: " + result.getDeltas().size() + " deltas found");
                }
                for (ContractDelta delta : result.getDeltas()) {
                    switch (delta.getSeverity()) {
                        case Error:
                            System.err.println("[ERROR] " + delta.getEntityType() + ": " + delta.getDescription());
                            break;
                        case Warning:
                            System.err.println("[WARNING] " + delta.getEntityType() + ": " + delta.getDescription());
                            break;
                        case Info:
                            System.out.println("[INFO] " + delta.getEntityType() + ": " + delta.getDescription());
                            break;
                    }
                }
                if (result.isCompatible() && result.getDeltas().isEmpty()) {
                    System.out.println("[INFO] Contract compatibility verified: client and server IDLs are identical");
                }
            }

            @Override
            public String name() {
                return "Logging";
            }
        };
    }

    static ContractAuditor failFast() {
        return new ContractAuditor() {
            @Override
            public void audit(VerificationResult result) {
                if (!result.isCompatible()) {
                    throw new RuntimeException("Contract compatibility verification failed: " + formatDeltas(result.getDeltas()));
                }
            }

            @Override
            public String name() {
                return "FailFast";
            }

            private String formatDeltas(java.util.List<ContractDelta> deltas) {
                if (deltas.isEmpty()) {
                    return "no deltas";
                }
                StringBuilder sb = new StringBuilder();
                for (ContractDelta delta : deltas) {
                    if (delta.getSeverity() == ContractDelta.Severity.Error) {
                        if (sb.length() > 0) {
                            sb.append("; ");
                        }
                        sb.append(delta.getEntityType()).append(": ").append(delta.getDescription());
                    }
                }
                return sb.length() > 0 ? sb.toString() : "no deltas";
            }
        };
    }
}