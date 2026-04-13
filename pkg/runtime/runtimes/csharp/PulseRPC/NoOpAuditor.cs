namespace PulseRPC
{
    public class NoOpAuditor : IContractAuditor
    {
        public void Audit(VerificationResult result)
        {
        }

        public string Name => "NoOp";
    }
}