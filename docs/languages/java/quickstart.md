---
title: Java Quickstart
layout: default
---

# Java Quickstart

Build a complete PulseRPC service in Java with our e-commerce checkout example.

## Prerequisites

- Java 11 or later
- Maven 3.6 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Define the Service (2 min)

Create `checkout.pulse` with your service definition:

{% include quickstart/checkout.idl %}

## 2. Generate Code (1 min)

Generate the Java code from your IDL:

```bash
pulserpc -plugin java-client-server -dir src/main/java -package com.example.myapp checkout.pulse
```

This creates:
- `src/main/java/com/example/myapp/checkout/` - Type definitions and Server/Client frameworks (package `com.example.myapp.checkout`)
- `src/main/java/pulserpc/` - Runtime library (package `pulserpc`)

The IDL is embedded directly in `Server.java` for the `pulserpc-idl` RPC method.

Create a `pom.xml` in your project root:

```xml
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>myapp</artifactId>
    <version>1.0-SNAPSHOT</version>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>com.fasterxml.jackson.core</groupId>
            <artifactId>jackson-databind</artifactId>
            <version>2.15.2</version>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.codehaus.mojo</groupId>
                <artifactId>exec-maven-plugin</artifactId>
                <version>3.1.0</version>
            </plugin>
        </plugins>
    </build>
</project>
```

## 3. Implement the Server (10-15 min)

Create `src/main/java/com/example/myapp/MyServer.java` that implements your service handlers:

{% include quickstart/java-server.md %}

Start your server:

```bash
mvn compile exec:java -Dexec.mainClass="com.example.myapp.MyServer"
```

## 4. Implement the Client (5-10 min)

Create `src/main/java/com/example/myapp/MyClient.java` to call your service:

{% include quickstart/java-client.md %}

Run your client:

```bash
mvn compile exec:java -Dexec.mainClass="com.example.myapp.MyClient"
```

## Error Codes

Throw `RPCError` with custom error codes:

```java
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

- [Java Reference](reference.html) - Type mappings and Jackson/GSon support
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
