namespace PulseRPC
{
    /// <summary>
    /// A single validation error carrying a field path and a message.
    /// </summary>
    public class ValidationError
    {
        public string Path { get; }
        public string Message { get; }

        public ValidationError(string path, string message)
        {
            Path = path ?? "";
            Message = message;
        }
    }
}
