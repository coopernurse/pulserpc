# Java Runtime Tests
# Requires: openjdk-17-jdk (or similar), maven

.PHONY: test clean test-validation test-types test-rpc test-json test-severity test-diff test-audit test-server test-envelope

# Test target - run all tests (java-12: include diff/auditor/severity suites)
test: test-validation test-types test-rpc test-json test-severity test-diff test-audit test-server test-envelope

# Test individual components
test-validation:
	@echo "Testing Java Validation..."
	@mvn clean test -Dtest=ValidationTest

test-types:
	@echo "Testing Java Types..."
	@mvn clean test -Dtest=TypesTest

test-rpc:
	@echo "Testing Java RPC..."
	@mvn clean test -Dtest=RPCTest

test-json:
	@echo "Testing Java JSON parsers..."
	@mvn clean test -Dtest=JsonParserTest

test-severity:
	@echo "Testing Java severity classification..."
	@mvn clean test -Dtest=SeverityTest

test-diff:
	@echo "Testing Java contract diff engine..."
	@mvn clean test -Dtest=DiffEngineTest

test-audit:
	@echo "Testing Java contract auditors..."
	@mvn clean test -Dtest=AuditorTest

test-server:
	@echo "Testing Java RPC server..."
	@mvn clean test -Dtest=ServerTest

test-envelope:
	@echo "Testing Java response envelope validation..."
	@mvn clean test -Dtest=EnvelopeTest

# Integration test - requires generated test server
test-integration:
	@echo "Running Java integration test..."
	# Build and start test server
	@mvn compile exec:java -Dexec.mainClass="TestServer" &
	@sleep 2
	# Run test client
	@mvn compile exec:java -Dexec.mainClass="TestClient"
	# Cleanup
	@pkill -f TestServer || true

# Clean target
clean:
	@mvn clean
	@rm -rf target/
	@rm -f *.class
