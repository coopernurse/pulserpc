# PulseRPC Quickstart Examples

This directory contains runnable quickstart examples for each supported language.

## Running Examples

### Go
```bash
cd go
make test
```

### Python
```bash
cd python
python3 server.py &
python3 client.py
```

### Java
```bash
cd java
mvn compile exec:java -Dexec.mainClass="MyServer" &
mvn compile exec:java -Dexec.mainClass="MyClient"
```

### TypeScript
```bash
cd typescript
npm install
npm run build &
node dist/my_server.js &
node dist/my_client.js
```

### C#
```bash
cd csharp/TestServer
dotnet run &
cd ../TestClient
dotnet run
```

## Automated Testing

Run all quickstart tests:
```bash
make test-quickstarts
```

Run individual language test:
```bash
make test-quickstart-go
```
