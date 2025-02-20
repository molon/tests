package reflect

import "reflect"

// Make creates a fully initialized instance of type T.
// For pointer types, it recursively initializes each level.
// For map, slice, and channel types, it returns initialized empty instances.
// For other types, it returns their zero values.
//
// It panics if:
// - T is a nil interface (type cannot be determined)
// - T is a function type (not supported)
func Make[T any]() T {
	var zero T
	t := reflect.TypeOf(zero)
	v := makeValue(t)
	return v.Interface().(T)
}

// makeValue recursively creates a new instance based on the given reflect.Type.
// For map, slice, and channel types, it returns initialized empty instances.
// For pointer types, it recursively creates the element instance and returns a pointer to it.
// For other types, it returns their zero values.
func makeValue(t reflect.Type) reflect.Value {
	if t == nil {
		panic("Make: cannot determine type from nil interface")
	}
	if t.Kind() == reflect.Func {
		panic("Make: function type is not supported")
	}

	switch t.Kind() {
	case reflect.Map:
		return reflect.MakeMap(t)
	case reflect.Slice:
		return reflect.MakeSlice(t, 0, 0)
	case reflect.Chan:
		return reflect.MakeChan(t, 0)
	}

	if t.Kind() != reflect.Ptr {
		return reflect.New(t).Elem()
	}

	elemType := t.Elem()
	elemValue := makeValue(elemType)
	ptrValue := reflect.New(elemType)
	ptrValue.Elem().Set(elemValue)
	return ptrValue
}
