package json

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	{
		var v any
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, map[string]interface{}{"a": float64(1)}, v) // 数据类型是 float64
	}

	type Foo struct {
		A int
		B int
	}

	{
		// v 本身就是具体类型时，当然 OK 啦，行为和预期一致
		v := Foo{B: 4}
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, Foo{A: 1, B: 4}, v)
	}
	{
		// 如果不给指针类型是会直接报错的，这句我先注释掉，是因为它总是静态分析警告，看着很讨厌
		// v := Foo{B: 4}
		// assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(non-pointer`)
	}
	{
		v := &Foo{B: 4}
		// 即使多取了两次地址，行为也是一致的
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), lo.ToPtr(&v)))
		assert.Equal(t, &Foo{A: 1, B: 4}, v)
	}
	{
		// 如果给空指针，并且取地址，也是很 OK 的
		var v *Foo
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), lo.ToPtr(&v)))
		assert.Equal(t, &Foo{A: 1}, v)
	}
	{
		// IMPORTANT: 如果给空指针，但未取地址，就不行了
		var v *Foo
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(nil`)

		var vv **Foo
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), vv), `json: Unmarshal(nil`)

		// 为什么给空指针再取一次地址就可以呢？
		// 因为 json.Unmarshal 里是通过 reflect.ValueOf(v).IsNil() 来判断的
		// 而可以理解为给空指针取地址得到的变量已经不是 nil 了，只是它的值是一个 nil 的指针，它不是 nil
		var vvv **Foo = func() **Foo {
			var a *Foo
			return &a
		}()
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), vvv))
	}

	// 以下测试如果 v 是 any 类型时的行为
	var v any
	{
		v = nil
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, map[string]interface{}{"a": float64(1)}, v)
	}
	{
		v = Foo{B: 8}
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		// IMPORTANT: 注意此时 v 已经不是 Foo 类型了，而是 map 类型，B 字段已经丢失
		// 如同忽略了 v = Foo{B: 8} 这一行，好像已经不是传入的那个它了
		assert.Equal(t, map[string]interface{}{"a": float64(1)}, v)
	}

	{
		v = Foo{}
		// 此时如果不给指针类型也是会报错的
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(non-pointer`)
	}

	{
		v = &Foo{B: 2}
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), v))
		// 注意这种情况下，v 是 *Foo 类型
		assert.Equal(t, &Foo{A: 1, B: 2}, v)
	}

	{
		// 如果传入到 Unmarshal 里又取了一次地址，这个倒是无所谓，行为和前者一致
		v = &Foo{B: 3}
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, &Foo{A: 1, B: 3}, v)
	}

	{
		// IMPORTANT: 注意如果 hold 的是空指针，返回的已经不是 *Foo 类型了，而是 map 类型
		// 如同忽略了 v = (*Foo)(nil) 这一行，好像已经不是传入的那个它了
		v = (*Foo)(nil)
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, map[string]interface{}{"a": float64(1)}, v)
	}

	{
		// IMPORTANT: 注意如果 hold 的是空指针，但未取地址，就一样不行，无论它有没有赋上具体类型
		v = (*Foo)(nil)
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(nil`)

		v = nil
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(nil`)
	}

	// IMPORTANT: 综上所述
	// 1. 传入参数时候取地址是最保险的
	// 2. 如果是通过 any hold 的话，需要确保 hold 的不能是 not-ptr / nil-ptr ，否则会丢失具体类型
}

// 这个范型方法可以直接规避掉上述问题，但是它只能是反序列化到一个空的结构体上
func Unmarshal[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	if err != nil {
		return v, err
	}
	return v, nil
}

func TestUnmarshalGeneric(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data := []byte(`{"name": "Alice", "age": 30}`)
	newPerson, err := Unmarshal[*Person](data)
	assert.NoError(t, err)
	assert.Equal(t, &Person{Name: "Alice", Age: 30}, newPerson)
}
