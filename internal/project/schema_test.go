package project

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Every key Load accepts is in the schema, and nothing else is, so the two
// can't drift apart.
func TestSchemaCoversEveryKey(t *testing.T) {
	s := Schema([]string{"go"}, []string{"github"}, `^[a-z.]+$`)
	var walk func(path string, typ reflect.Type, schema map[string]any)
	walk = func(path string, typ reflect.Type, schema map[string]any) {
		props, _ := schema["properties"].(map[string]any)
		var keys []string
		for i := range typ.NumField() {
			f := typ.Field(i)
			key, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
			keys = append(keys, key)
			sub, ok := props[key].(map[string]any)
			if !ok {
				t.Errorf("%s%s isn't in the schema", path, key)
				continue
			}
			switch ft := f.Type; {
			case ft.Kind() == reflect.Struct:
				walk(path+key+".", ft, sub)
			case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
				walk(path+key+"[].", ft.Elem(), sub["items"].(map[string]any))
			}
		}
		for key := range props {
			if !slices.Contains(keys, key) {
				t.Errorf("the schema has %s%s, which Load doesn't accept", path, key)
			}
		}
	}
	walk("", reflect.TypeFor[File](), s)
}
