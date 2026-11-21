using System;
using System.Collections.Generic;

namespace PulseRPC
{
    public class VerificationResult
    {
        public bool Compatible { get; init; }
        /// <summary>Non-empty when verification could not be completed (inconclusive).</summary>
        public string Error { get; init; } = string.Empty;
        public string ServerChecksum { get; init; } = string.Empty;
        public string ClientChecksum { get; init; } = string.Empty;
        public IReadOnlyList<ContractDelta> Deltas { get; init; } = Array.Empty<ContractDelta>();
        public DateTime Timestamp { get; init; }
    }
}