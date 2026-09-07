package project

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

func decodeDetails(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	err := d.Decode(&value)
	return value, err
}

func checkAuditPrefix(old, current []HistoryRecord) error {
	if len(current) < len(old) {
		return fmt.Errorf("save would truncate persisted audit history; use saveas to a new path")
	}
	for i, a := range old {
		b := current[i]
		av, err := decodeDetails(a.Extra)
		if err != nil {
			return err
		}
		bv, err := decodeDetails(b.Extra)
		if err != nil {
			return err
		}
		sameIDs := func(a, b []string) bool { return len(a) == len(b) && stringsEqual(a, b) }
		if !a.At.Equal(b.At) || a.Command != b.Command || a.Result != b.Result || a.Error != b.Error || !sameIDs(a.Created, b.Created) || !sameIDs(a.Updated, b.Updated) || !sameIDs(a.Deleted, b.Deleted) || !reflect.DeepEqual(av, bv) || (len(a.Extra) == 0) != (len(b.Extra) == 0) {
			return fmt.Errorf("save would rewrite persisted audit entry %d; reopen the latest project or use saveas to a new path", i+1)
		}
	}
	return nil
}
func stringsEqual(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writeAuditValues(w *rowWriter, event int, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	value, err := decodeDetails(raw)
	if err != nil {
		return err
	}
	next := 0
	var visit func(any, any, int, string, int)
	visit = func(v any, parent any, ordinal int, member string, depth int) {
		if w.err != nil {
			return
		}
		if depth > 1000 {
			w.err = fmt.Errorf("excessively deep audit details")
			return
		}
		node := next
		next++
		kind := "null"
		var scalar any
		switch v := v.(type) {
		case map[string]any:
			kind = "object"
		case []any:
			kind = "array"
		case string:
			kind = "string"
			scalar = v
		case json.Number:
			kind = "number"
			scalar = v.String()
		case bool:
			kind = "boolean"
			scalar = strconv.FormatBool(v)
		}
		w.insert("audit_values", event, node, parent, ordinal, member, kind, scalar)
		switch v := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, k := range keys {
				visit(v[k], node, i, k, depth+1)
			}
		case []any:
			for i, item := range v {
				visit(item, node, i, "", depth+1)
			}
		}
	}
	visit(value, nil, 0, "", 0)
	return w.err
}

type auditNode struct {
	id           int
	parent       sql.NullInt64
	ordinal      int
	member, kind string
	value        sql.NullString
}

func readAuditValues(q sqlReader, event int) (json.RawMessage, error) {
	nodes := map[int]auditNode{}
	children := map[int][]int{}
	root := -1
	err := eachRow(q, "SELECT node,parent,ordinal,member,kind,value FROM audit_values WHERE event_id=? ORDER BY ordinal", func(r *sql.Rows) error {
		var n auditNode
		if err := r.Scan(&n.id, &n.parent, &n.ordinal, &n.member, &n.kind, &n.value); err != nil {
			return err
		}
		nodes[n.id] = n
		if n.parent.Valid {
			children[int(n.parent.Int64)] = append(children[int(n.parent.Int64)], n.id)
		} else {
			if root != -1 {
				return fmt.Errorf("multiple audit detail roots")
			}
			root = n.id
		}
		return nil
	}, event)
	if err != nil || len(nodes) == 0 {
		return nil, err
	}
	if root < 0 {
		return nil, fmt.Errorf("missing audit detail root")
	}
	seen := map[int]bool{}
	var visit func(int, int) (any, error)
	visit = func(id, depth int) (any, error) {
		if seen[id] || depth > 1000 {
			return nil, fmt.Errorf("cyclic or excessively deep audit details")
		}
		seen[id] = true
		n, ok := nodes[id]
		if !ok {
			return nil, fmt.Errorf("missing audit detail node")
		}
		child := children[id]
		if n.kind != "array" && n.kind != "object" && len(child) > 0 {
			return nil, fmt.Errorf("scalar audit detail has children")
		}
		switch n.kind {
		case "object", "array":
			obj := map[string]any{}
			array := []any{}
			for i, c := range child {
				if nodes[c].ordinal != i {
					return nil, fmt.Errorf("unordered audit details")
				}
				v, err := visit(c, depth+1)
				if err != nil {
					return nil, err
				}
				if n.kind == "array" {
					array = append(array, v)
				} else {
					key := nodes[c].member
					if _, ok := obj[key]; ok {
						return nil, fmt.Errorf("duplicate audit member")
					}
					obj[key] = v
				}
			}
			if n.kind == "array" {
				return array, nil
			}
			return obj, nil
		case "string":
			if !n.value.Valid {
				return nil, fmt.Errorf("missing string value")
			}
			return n.value.String, nil
		case "number":
			if !n.value.Valid {
				return nil, fmt.Errorf("missing number")
			}
			return json.Number(n.value.String), nil
		case "boolean":
			return strconv.ParseBool(n.value.String)
		case "null":
			return nil, nil
		}
		return nil, fmt.Errorf("invalid audit node kind %q", n.kind)
	}
	value, err := visit(root, 0)
	if err != nil {
		return nil, err
	}
	if len(seen) != len(nodes) {
		return nil, fmt.Errorf("disconnected audit details")
	}
	return json.Marshal(value)
}
