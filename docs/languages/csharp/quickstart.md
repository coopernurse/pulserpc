---
title: C# Quickstart
layout: default
---

# C# Quickstart

Build a complete PulseRPC service in C# with our e-commerce checkout example.

## Prerequisites

- .NET 8.0 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Define the Service (2 min)

Create `checkout.pulse` with your service definition:

{% include quickstart/checkout.idl %}

## 2. Generate Code (1 min)

Generate the C# code from your IDL:

```bash
pulserpc -plugin csharp-client-server checkout.pulse
```

This creates:
- `Checkout.cs` - Type definitions (in `checkout` namespace)
- `Server.cs` - RPC server framework (in `PulseRPC` namespace)
- `Client.cs` - RPC client framework (in `PulseRPC` namespace)
- `Contract.cs` - Shared interfaces and IDL metadata
- `PulseRPC/` - Runtime library

**Pro tip:** Organize your generated code into a `Shared/` directory to keep things tidy:

```bash
mkdir Shared TestServer TestClient
mv Checkout.cs Client.cs Contract.cs Server.cs PulseRPC/ Shared/
```

## Multi-Namespace Support

When generating code with multiple namespaces (e.g., `common`, `book`, `user`), use the `-package` flag to set a base namespace prefix:

```bash
pulserpc -plugin csharp-client-server -dir ./output -package MyApp.Lib.Rpc common.pulse book.pulse user.pulse
```

This creates the following output structure:

```
output/
├── Common/
│   ├── Types.cs
│   ├── Server.cs
│   └── Client.cs
├── Book/
│   ├── Types.cs
│   ├── Server.cs
│   └── Client.cs
├── User/
│   ├── Types.cs
│   ├── Server.cs
│   └── Client.cs
├── PulseRPC/
│   └── (runtime files)
├── Contract.cs
├── Server.cs
└── Client.cs
```

Generated code will use:
- Cross-namespace imports: `using MyApp.Lib.Rpc.Common;`
- Runtime imports: `using MyApp.Lib.Rpc.PulseRPC;`

Without `-package`, the plugin uses the default `PulseRPC` namespace (backwards compatible).

## 3. Implement the Server (10-15 min)

Create a server project file `TestServer/TestServer.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
  </PropertyGroup>

  <ItemGroup>
    <FrameworkReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>

  <ItemGroup>
    <Compile Include="../Shared/Checkout/Checkout.cs" />
    <Compile Include="../Shared/Contract.cs" />
    <Compile Include="../Shared/Server.cs" />
    <Compile Include="../Shared/PulseRPC/*.cs" />
  </ItemGroup>

</Project>
```

Create `TestServer/MyServer.cs` that implements your service handlers:

{% include quickstart/csharp-server.md %}

Start your server:

```bash
cd TestServer
dotnet run
```

## 4. Implement the Client (5-10 min)

Create a client project file `TestClient/TestClient.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
  </PropertyGroup>

  <ItemGroup>
    <FrameworkReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>

  <ItemGroup>
    <Compile Include="../Shared/Checkout/Checkout.cs" />
    <Compile Include="../Shared/Contract.cs" />
    <Compile Include="../Shared/Client.cs" />
    <Compile Include="../Shared/PulseRPC/*.cs" />
  </ItemGroup>

</Project>
```

Create `TestClient/MyClient.cs` to call your service:

{% include quickstart/csharp-client.md %}

Run your client:

```bash
cd TestClient
dotnet run
```

## Error Codes

Throw `RPCError` with custom error codes:

```csharp
throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");
```

| Code | Name |
|------|------|
| 1001 | CartNotFound |
| 1002 | CartEmpty |
| 1003 | PaymentFailed |
| 1004 | OutOfStock |
| 1005 | InvalidAddress |

## Next Steps

- [C# Reference](reference.html) - Type mappings and async/await patterns
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
