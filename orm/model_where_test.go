package orm

import (
	"reflect"
	"testing"
)

func TestBuildWhereClauseUsesInForTypedSlices(t *testing.T) {
	clause, args := buildWhereClause(map[string]any{"id": []uint64{1, 2}})

	if clause != "id IN (?,?)" {
		t.Fatalf("unexpected clause: %s", clause)
	}

	expectedArgs := []any{uint64(1), uint64(2)}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildWhereClauseKeepsBytesAsSingleValue(t *testing.T) {
	value := []byte{1, 2}
	clause, args := buildWhereClause(map[string]any{"blob": value})

	if clause != "blob = ?" {
		t.Fatalf("unexpected clause: %s", clause)
	}

	if len(args) != 1 {
		t.Fatalf("unexpected args length: %d", len(args))
	}

	got, ok := args[0].([]byte)
	if !ok {
		t.Fatalf("unexpected arg type: %T", args[0])
	}

	if !reflect.DeepEqual(got, value) {
		t.Fatalf("unexpected arg value: %#v", got)
	}
}
