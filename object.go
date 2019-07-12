package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/francoispqt/gojay"
)

type k8Object struct {
	// Whole object in original format
	raw gojay.EmbeddedJSON

	namespace string
	kind      string
}

func (obj k8Object) Namespace() string {
	if len(obj.namespace) == 0 {
		// https://github.com/kubernetes/kubernetes/blob/v1.6.6/pkg/api/types.go#L83
		//
		// "An empty namespace is equivalent to the "default" namespace, but
		// "default" is the canonical representation. Not all objects are
		// required to be scoped to a namespace - the value of this field for
		// those objects will be empty."
		return "default"
	}

	return obj.namespace
}

func (obj k8Object) Kind() string {
	if len(obj.kind) == 0 {
		return "_unknown_"
	}

	return obj.kind
}

func (obj k8Object) NKeys() int {
	return 0
}

func (obj *k8Object) UnmarshalJSONObject(dec *gojay.Decoder, k string) error {
	switch k {
	case "kind":
		return dec.String(&obj.kind)

	case "metadata":
		return dec.Object(gojay.DecodeObjectFunc(func(dec *gojay.Decoder, k string) error {
			switch k {
			case "namespace":
				return dec.String(&obj.namespace)
			}

			return nil
		}))
	}

	return nil
}

type k8ListItems []*k8Object

// Load an array of Kubernetes objects. Each is re-indented for consistent
// formatting.
func (items *k8ListItems) UnmarshalJSONArray(dec *gojay.Decoder) error {
	raw := gojay.EmbeddedJSON{}

	if err := dec.EmbeddedJSON(&raw); err != nil {
		return err
	}

	// Reindent
	indented := bytes.Buffer{}

	if err := json.Indent(&indented, []byte(raw), "", "  "); err != nil {
		return err
	}

	obj := &k8Object{
		raw: gojay.EmbeddedJSON(indented.Bytes()),
	}

	// Extract object information
	if err := gojay.UnmarshalJSONObject(obj.raw, obj); err != nil {
		return err
	}

	*items = append(*items, obj)

	return nil
}

type k8List struct {
	items k8ListItems
}

func (l *k8List) NKeys() int {
	return 0
}

func (l *k8List) UnmarshalJSONObject(dec *gojay.Decoder, k string) error {
	switch k {
	case "apiVersion", "metadata":
		return nil

	case "kind":
		var kind string
		if err := dec.String(&kind); err != nil {
			return err
		}

		if kind != "List" {
			return errors.New("Expected list object")
		}

		return nil

	case "items":
		return dec.Array(&l.items)
	}

	return fmt.Errorf("Unrecognized object key %q", k)
}
