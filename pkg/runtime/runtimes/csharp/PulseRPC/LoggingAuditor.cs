using System;

namespace PulseRPC
{
    public class LoggingAuditor : IContractAuditor
    {
        public void Audit(VerificationResult result)
        {
            if (!result.Compatible)
            {
                Console.Error.WriteLine($"[ERROR] Contract incompatibility detected: {result.Deltas.Count} deltas found");
            }

            foreach (var delta in result.Deltas)
            {
                switch (delta.Severity)
                {
                    case Severity.Error:
                        Console.Error.WriteLine($"[ERROR] {delta.EntityType}: {delta.Description}");
                        break;
                    case Severity.Warning:
                        Console.Error.WriteLine($"[WARNING] {delta.EntityType}: {delta.Description}");
                        break;
                    case Severity.Info:
                        Console.WriteLine($"[INFO] {delta.EntityType}: {delta.Description}");
                        break;
                }
            }

            if (result.Compatible && result.Deltas.Count == 0)
            {
                Console.WriteLine("[INFO] Contract compatibility verified: client and server IDLs are identical");
            }
        }

        public string Name => "Logging";
    }
}