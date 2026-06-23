package excedo

import "testing"

func TestDecodeJSONStrictRejectsUnknownField(t *testing.T) {
	input := []byte(`{"code":1000,"message":"ok","token":"abc","extra":1}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err == nil {
		t.Fatalf("expected unknown field error")
	}
}

func TestDecodeJSONStrictRejectsTrailingContent(t *testing.T) {
	input := []byte(`{"code":1000,"message":"ok","token":"abc"} {}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err == nil {
		t.Fatalf("expected trailing content error")
	}
}
