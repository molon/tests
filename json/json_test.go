package json

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	{
		var v any
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		assert.Equal(t, map[string]any{"a": float64(1)}, v) // 数据类型是 float64
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
		assert.Equal(t, map[string]any{"a": float64(1)}, v)
	}
	{
		v = Foo{B: 8}
		assert.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &v))
		// IMPORTANT: 注意此时 v 已经不是 Foo 类型了，而是 map 类型，B 字段已经丢失
		// 如同忽略了 v = Foo{B: 8} 这一行，好像已经不是传入的那个它了
		assert.Equal(t, map[string]any{"a": float64(1)}, v)
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
		assert.Equal(t, map[string]any{"a": float64(1)}, v)
	}

	{
		// IMPORTANT: 注意如果 hold 的是空指针，但未取地址，就一样不行，无论它有没有赋上具体类型
		v = (*Foo)(nil)
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(nil`)

		v = nil
		assert.ErrorContains(t, json.Unmarshal([]byte(`{"a":1}`), v), `json: Unmarshal(nil`)
	}

	{
		v = []int{} // 虽然指定了具体类型，但是它还是属于 any hold 的 not-ptr 类型，所以会丢失具体类型
		assert.NoError(t, json.Unmarshal([]byte(`[1,2,3]`), &v))
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, v)

		v = (*[]int)(nil) // 虽然指定了具体类型，但是它还是属于 any hold 的 nil-ptr 类型，所以会丢失具体类型
		assert.NoError(t, json.Unmarshal([]byte(`[1,2,3]`), &v))
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, v)
	}

	{
		v = lo.ToPtr([]int{}) // 这就 hold 的是一个 not-nil-ptr 类型，所以不会丢失具体类型
		assert.NoError(t, json.Unmarshal([]byte(`[1,2,3]`), &v))
		assert.Equal(t, lo.ToPtr([]int{1, 2, 3}), v)
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

type A struct {
	ID string `json:"id,omitempty"`
}

func (v *A) GetID() string {
	return v.ID
}

type B struct {
	ID string `json:"id,omitempty"`
}

func (v *B) GetID() string {
	return v.ID
}

type C struct {
	A
}

type D struct {
	A
	B
}

type Identifiable interface {
	GetID() string
}

type E struct {
	A
	Identifiable
}

type F struct {
	Name string `json:"name"`
}

type G struct {
	A
	F
}

type H struct {
	F
	Identifiable
}

func TestDuplicateFieldName(t *testing.T) {
	{
		result, err := json.Marshal(C{A: A{ID: "a"}})
		require.NoError(t, err)
		if !assert.JSONEq(t, `{"id":"a"}`, string(result)) {
			t.Log(string(result))
		}
	}
	{
		result, err := json.Marshal(D{A: A{ID: "a"}, B: B{ID: "b"}})
		require.NoError(t, err)
		if !assert.JSONEq(t, `{}`, string(result)) {
			t.Log(string(result))
		}
		// 以上说明存在 embed 里同名的话，其实俩都会被结果里忽略
	}
	{
		result, err := json.Marshal(E{A: A{ID: "a"}, Identifiable: &B{ID: "b"}})
		require.NoError(t, err)
		if !assert.JSONEq(t, `{"Identifiable":{"id":"b"},"id":"a"}`, string(result)) {
			t.Log(string(result))
		}
		// 以上说明 embed 一个 interface 的话，最终其不会把字段作为 embed 的 json 字段
	}
	{
		result, err := json.Marshal(G{A: A{ID: "a"}, F: F{Name: "f"}})
		require.NoError(t, err)
		if !assert.JSONEq(t, `{"id":"a","name":"f"}`, string(result)) {
			t.Log(string(result))
		}
		// 以上说明 embed 一个非 interface 的话，确实还是会把字段作为 embed 的 json 字段
	}
	{
		result, err := json.Marshal(H{F: F{Name: "f"}, Identifiable: &B{ID: "b"}})
		require.NoError(t, err)
		if !assert.JSONEq(t, `{"name":"f","Identifiable":{"id":"b"}}`, string(result)) {
			t.Log(string(result))
		}
		// 以上说明 embed 一个 interface 的话，无论其是否有同名字段，都不会把 interface 字段作为 embed 的 json 字段
	}
}
