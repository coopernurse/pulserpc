using System;
using System.Linq;
using System.Text;

namespace PulseRPC
{
    public class FailFastAuditor : IContractAuditor
    {
        public void Audit(VerificationResult result)
        {
            if (!result.Compatible)
            {
                throw new InvalidOperationException("Contract compatibility verification failed: " + FormatDeltas(result.Deltas));
            }
        }

        public string Name => "FailFast";

        private static string FormatDeltas(IReadOnlyList<ContractDelta> deltas)
        {
            if (deltas.Count == 0)
            {
                return "no deltas";
            }

            var sb = new StringBuilder();
            foreach (var delta in deltas)
            {
                if (delta.Severity == Severity.Error)
                {
                    if (sb.Length > 0)
                    {
                        sb.Append("; ");
                    }
                    sb.Append($"{delta.EntityType}: {delta.Description}");
                }
            }
            return sb.Length > 0 ? sb.ToString() : "no deltas";
        }
    }
}