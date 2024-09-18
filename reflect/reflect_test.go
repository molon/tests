package reflect

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressable(t *testing.T) {
	{
		// reflect.Value 分为可寻址和不可寻址
		var a int
		// 不可寻址
		assert.False(t, reflect.ValueOf(a).CanAddr())
		// 可寻址，可以理解为 &a 已经是一个新的变量，为 a 的指针，所以可以寻址
		// 为什么要加 Elem()，因为 &a 是一个指针，Elem() 取出指针指向的值才算是和 a 匹配
		assert.True(t, reflect.ValueOf(&a).Elem().CanAddr())

		// 不可寻址的值，不能进行 Set 操作
		assert.Panics(t, func() {
			reflect.ValueOf(a).SetInt(1)
		})
		// 可寻址的值，可以进行 Set 操作
		assert.NotPanics(t, func() {
			reflect.ValueOf(&a).Elem().SetInt(1)
		})
	}

	{
		// reflect.Zero 是不可寻址的，所以也不可进行 Set 操作
		zero := reflect.Zero(reflect.TypeOf(1))
		assert.Panics(t, func() {
			zero.SetInt(1)
		})
		iface := zero.Interface()
		assert.Equal(t, 0, iface)
		// 将其搞到一个指针上，就可以进行 Set 操作
		assert.NotPanics(t, func() {
			reflect.ValueOf(&iface).Elem().Set(reflect.ValueOf(2))
		})
	}

	{
		// 那如果 reflect.Zero 本身就是一个指针呢？
		var a int = 1
		zero := reflect.Zero(reflect.TypeOf(&a))
		iface := zero.Interface().(*int)
		// 这样不行，因为它是一个 nil 的 *int 指针
		assert.Panics(t, func() {
			*iface = 2
		})
		// 和前者类似，因为它本身其实就没有 Elem ，其 Elem 会是一个 zero value
		assert.Panics(t, func() {
			zero.Elem().SetInt(2)
		})
	}
}

func TestTypeOf(t *testing.T) {
	// 总的来说，TypeOf 和 ValueOf.Type() 行为一致，但是对于 nil interface 的情况有些许特别
	{
		// nil interface 的 TypeOf 是 nil ，ValueOf 是 invalid value ，无法执行 Type()
		var i any
		assert.Nil(t, reflect.TypeOf(i))
		assert.False(t, reflect.ValueOf(i).IsValid())
		assert.Panics(t, func() {
			reflect.ValueOf(i).Type()
		})
	}

	{
		// nil pointer 的 TypeOf 是 *int ，ValueOf.Type 是 *int ，行为一致
		var p *int
		assert.Equal(t, reflect.TypeOf(p), reflect.TypeOf((*int)(nil)))
		assert.Equal(t, reflect.TypeOf(p), reflect.ValueOf(p).Type())
	}

	{
		// interface holding a nil pointer 的 TypeOf 是 *int ，ValueOf.Type 是 *int ，行为一致
		var p *int
		var iface any = p
		assert.Equal(t, reflect.TypeOf(iface), reflect.TypeOf((*int)(nil)))
		assert.Equal(t, reflect.TypeOf(iface), reflect.ValueOf(iface).Type())
	}

	{
		// non-nil interface holding an int 的 TypeOf 是 int ，ValueOf.Type 是 int ，行为一致
		var val int = 42
		var iface any = val
		assert.Equal(t, reflect.TypeOf(iface), reflect.TypeOf(1))
		assert.Equal(t, reflect.TypeOf(iface), reflect.ValueOf(iface).Type())
	}
}

// UnmarshalToNew unmarshals JSON data into a new value of the same type as v.
func UnmarshalToNew(data []byte, v any) (any, error) {
	// Get the type of v. If v is a nil value with no type (e.g., nil interface), vType will be nil.
	vType := reflect.TypeOf(v)
	if vType == nil {
		// If v is a nil value with no type, unmarshal into a map[string]interface{}.
		var cp any
		err := json.Unmarshal(data, &cp)
		if err != nil {
			return nil, err
		}
		return cp, nil
	}

	// Create a new instance of v's type. cp will be of the same type as v.
	// 注意这是一个好的方式，如果使用 reflect.Zero(vType) 会导致不可寻址，而这个是可寻址的，且刚好满足了 json.Unmarshal 的取地址
	cp := reflect.New(vType).Elem()
	err := json.Unmarshal(data, cp.Addr().Interface())
	if err != nil {
		return nil, err
	}

	return cp.Interface(), nil
}

func TestUnmarshalToNew(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	{
		// 传入一个值，返回一个新的值
		data := []byte(`{"name":"Alice","age":30}`)
		var p Person
		newP, err := UnmarshalToNew(data, p)
		assert.NoError(t, err)
		assert.Equal(t, Person{Name: "Alice", Age: 30}, newP)
		assert.NotEqual(t, p, newP)
	}

	{
		// 传入一个指针，返回一个新的指针
		data := []byte(`{"name":"Alice","age":30}`)
		var p *Person
		newP, err := UnmarshalToNew(data, p)
		assert.NoError(t, err)
		assert.Equal(t, &Person{Name: "Alice", Age: 30}, newP)
		assert.NotEqual(t, p, newP)
	}

	{
		// 如果传入的值是 nil interface ，会返回 map[string]any , 和 json.Unmarshal 的默认行为保持一致
		data := []byte(`{"name":"Alice","age":30}`)
		var p any
		newP, err := UnmarshalToNew(data, p)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"age": float64(30), "name": "Alice"}, newP)
		assert.NotEqual(t, p, newP)
	}
}
