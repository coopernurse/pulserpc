package pulserpc

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyCompatibility_IdenticalIDLs(t *testing.T) {
	idl := `{
		"interfaces": [{
			"name": "TestService",
			"methods": [{
				"name": "testMethod",
				"parameters": [{"name": "arg1", "type": "string"}],
				"returnType": {"type": "string"}
			}]
		}],
		"structs": [{
			"name": "TestStruct",
			"fields": [{"name": "field1", "type": "string"}]
		}]
	}`

	var clientIDL, serverIDL interface{}
	if err := json.Unmarshal([]byte(idl), &clientIDL); err != nil {
		t.Fatalf("failed to parse client IDL: %v", err)
	}
	if err := json.Unmarshal([]byte(idl), &serverIDL); err != nil {
		t.Fatalf("failed to parse server IDL: %v", err)
	}

	deltas := DiffIDL(clientIDL, serverIDL)

	compatible := true
	for _, delta := range deltas {
		if delta.Severity == SeverityError {
			compatible = false
			break
		}
	}

	if !compatible {
		t.Error("expected compatible=true for identical IDLs")
	}
	if len(deltas) != 0 {
		t.Errorf("expected 0 deltas, got %d", len(deltas))
	}
}

func TestVerifyCompatibility_AddedOptionalField(t *testing.T) {
	clientIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false}
			]
		}]
	}`

	serverIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false},
				{"name": "newField", "type": "int", "optional": true}
			]
		}]
	}`

	var client, server interface{}
	json.Unmarshal([]byte(clientIDL), &client)
	json.Unmarshal([]byte(serverIDL), &server)

	deltas := DiffIDL(client, server)

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}

	if deltas[0].Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for added optional field, got %s", deltas[0].Severity)
	}

	if deltas[0].ChangeType != ChangeAdded {
		t.Errorf("expected ChangeType Added, got %s", deltas[0].ChangeType)
	}

	if deltas[0].Direction != DirectionClientHasLess {
		t.Errorf("expected Direction ClientHasLess, got %s", deltas[0].Direction)
	}
}

func TestVerifyCompatibility_AddedRequiredField(t *testing.T) {
	clientIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false}
			]
		}]
	}`

	serverIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false},
				{"name": "newField", "type": "int", "optional": false}
			]
		}]
	}`

	var client, server interface{}
	json.Unmarshal([]byte(clientIDL), &client)
	json.Unmarshal([]byte(serverIDL), &server)

	deltas := DiffIDL(client, server)

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}

	if deltas[0].Severity != SeverityError {
		t.Errorf("expected SeverityError for added required field, got %s", deltas[0].Severity)
	}
}

func TestVerifyCompatibility_RemovedRequiredField(t *testing.T) {
	clientIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false},
				{"name": "oldField", "type": "int", "optional": false}
			]
		}]
	}`

	serverIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "existingField", "type": "string", "optional": false}
			]
		}]
	}`

	var client, server interface{}
	json.Unmarshal([]byte(clientIDL), &client)
	json.Unmarshal([]byte(serverIDL), &server)

	deltas := DiffIDL(client, server)

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}

	if deltas[0].Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for removed field, got %s", deltas[0].Severity)
	}

	if deltas[0].Direction != DirectionClientHasMore {
		t.Errorf("expected Direction ClientHasMore, got %s", deltas[0].Direction)
	}
}

func TestVerifyCompatibility_FieldMadeOptional(t *testing.T) {
	clientIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "status", "type": "string", "optional": false}
			]
		}]
	}`

	serverIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "status", "type": "string", "optional": true}
			]
		}]
	}`

	var client, server interface{}
	json.Unmarshal([]byte(clientIDL), &client)
	json.Unmarshal([]byte(serverIDL), &server)

	deltas := DiffIDL(client, server)

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}

	if deltas[0].Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for field made optional, got %s", deltas[0].Severity)
	}

	if deltas[0].ChangeType != ChangeModified {
		t.Errorf("expected ChangeType Modified, got %s", deltas[0].ChangeType)
	}
}

func TestVerifyCompatibility_FieldMadeRequired(t *testing.T) {
	clientIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "status", "type": "string", "optional": true}
			]
		}]
	}`

	serverIDL := `{
		"interfaces": [],
		"structs": [{
			"name": "TestStruct",
			"fields": [
				{"name": "status", "type": "string", "optional": false}
			]
		}]
	}`

	var client, server interface{}
	json.Unmarshal([]byte(clientIDL), &client)
	json.Unmarshal([]byte(serverIDL), &server)

	deltas := DiffIDL(client, server)

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}

	if deltas[0].Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for field made required, got %s", deltas[0].Severity)
	}

	if deltas[0].ChangeType != ChangeModified {
		t.Errorf("expected ChangeType Modified, got %s", deltas[0].ChangeType)
	}
}

func TestVerifyCompatibility_Checksums(t *testing.T) {
	idl := `{"test": "data"}`
	var idlData interface{}
	json.Unmarshal([]byte(idl), &idlData)

	checksum := ComputeChecksum(idlData)

	if checksum == "" {
		t.Error("expected non-empty checksum")
	}

	checksum2 := ComputeChecksum(idlData)
	if checksum != checksum2 {
		t.Error("expected same IDL to produce same checksum")
	}

	differentIDL := `{"different": "data"}`
	var differentData interface{}
	json.Unmarshal([]byte(differentIDL), &differentData)

	checksum3 := ComputeChecksum(differentData)
	if checksum == checksum3 {
		t.Error("expected different IDLs to produce different checksums")
	}
}

func TestVerifyCompatibility_VerificationResult(t *testing.T) {
	idl := `{"interfaces": [], "structs": [], "enums": []}`
	var idlData interface{}
	json.Unmarshal([]byte(idl), &idlData)

	deltas := DiffIDL(idlData, idlData)

	compatible := true
	for _, delta := range deltas {
		if delta.Severity == SeverityError {
			compatible = false
			break
		}
	}

	result := &VerificationResult{
		Compatible:     compatible,
		ServerChecksum: ComputeChecksum(idlData),
		ClientChecksum: ComputeChecksum(idlData),
		Deltas:         deltas,
		Timestamp:      time.Now(),
	}

	if !result.Compatible {
		t.Error("expected Compatible=true")
	}

	if result.ServerChecksum == "" {
		t.Error("expected non-empty ServerChecksum")
	}

	if result.ClientChecksum == "" {
		t.Error("expected non-empty ClientChecksum")
	}

	if len(result.Deltas) != 0 {
		t.Errorf("expected empty Deltas, got %d", len(result.Deltas))
	}
}
