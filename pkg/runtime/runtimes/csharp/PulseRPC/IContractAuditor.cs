namespace PulseRPC
{
    public interface IContractAuditor
    {
        void Audit(VerificationResult result);
        string Name { get; }
    }
}