package reflect

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/samber/lo"
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
		assert.Equal(t, 2, iface)
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

func TestElem(t *testing.T) {
	{
		// reflect.Value.Elem() 取出指针指向的值
		var a int = 1
		var p *int = &a
		assert.Equal(t, 1, reflect.ValueOf(p).Elem().Interface())
	}

	{
		// reflect.Type.Elem() 取出指针指向的类型
		var a int = 1
		var p *int = &a
		assert.Equal(t, reflect.TypeOf(a), reflect.TypeOf(p).Elem())
	}
}

func TestNew(t *testing.T) {
	{
		// reflect.New() 创建一个指针
		var a int
		p := reflect.New(reflect.TypeOf(a))
		assert.Equal(t, 0, p.Elem().Interface())
	}

	{
		// reflect.New() 创建一个指向指针的指针
		var a int
		p := reflect.New(reflect.TypeOf(&a))
		assert.Nil(t, p.Elem().Interface())
		assert.True(t, p.Elem().IsNil())             // 指向一个空指针的指针，IsNil 为 true
		assert.False(t, p.Elem().Interface() == nil) // 指向一个空指针的指针和 nil 直接判断，并非 nil
	}

	{
		// reflect.New() 创建一个相同类型的空值，这样比使用 reflect.Zero() 更舒服，因为这样是可寻址的
		var a int = 1
		p := reflect.New(reflect.TypeOf(a)).Elem()
		assert.Equal(t, 0, p.Interface())
	}

	{
		type Foo struct {
			A int
			B string
		}
		var a *Foo
		p := reflect.New(reflect.TypeOf(a)).Elem()
		b := p.Interface().(*Foo)
		assert.Nil(t, b) // 这里 b 是 nil ，因为 New 创建的是一个空指针

		{
			p := reflect.New(reflect.TypeOf(a).Elem()).Elem()
			b := p.Interface().(Foo)
			assert.Equal(t, Foo{A: 0, B: ""}, b)
		}
	}
}

func TestMakeSlice(t *testing.T) {
	{
		// 使用 reflect.MakeSlice 创建一个切片
		sliceType := reflect.SliceOf(reflect.TypeOf(int(0)))
		slice := reflect.MakeSlice(sliceType, 3, 3) // 创建一个长度为 3 的切片

		// 获取 reflect.MakeSlice 创建的切片的真实切片
		normalSlice := slice.Interface().([]int)

		// 现在可以修改切片的元素
		normalSlice[0] = 100
		normalSlice[1] = 200
		normalSlice[2] = 300

		// 输出修改后的切片
		assert.Equal(t, []int{100, 200, 300}, normalSlice)
		assert.Equal(t, []int{100, 200, 300}, slice.Interface())

		ptr := &normalSlice
		(*ptr)[1] = 400
		assert.Equal(t, []int{100, 400, 300}, normalSlice)
		assert.Equal(t, []int{100, 400, 300}, slice.Interface())
	}
	{
		sliceType := reflect.TypeOf([]int{})
		slice := reflect.MakeSlice(sliceType, 0, 0)
		assert.Panics(t, func() {
			_ = slice.Addr().Interface() // MakeSlice 出来的玩意不可寻址
		})
	}
	{
		sliceType := reflect.TypeOf([]int{})
		sliceMaked := reflect.MakeSlice(sliceType, 0, 0)
		slice := reflect.New(sliceType).Elem()
		// t.Logf("sliceMaked: %#v", sliceMaked.Interface())
		t.Logf("slice: %#v", slice.Interface())
		assert.True(t, slice.IsNil()) // 直接 New 出来的 slice 是 nil
		slice.Set(sliceMaked)
		assert.False(t, slice.IsNil()) // Set 一个 MakeSlice 才能变得不是 nil
		t.Logf("slicePtr: %#v", slice.Interface())
	}
	{
		a := reflect.MakeSlice(reflect.TypeOf([]int{}), 0, 0).Interface() // 这里 a 是 []int 类型
		t.Logf("a: %T", a)
		err := json.Unmarshal([]byte(`[1,2,3]`), &a)
		assert.NoError(t, err)
		t.Logf("a: %T", a)
		// 注意这里类型变了，是因为如果是通过 any hold 的话，需要确保 hold 的不能是 not-ptr / nil-ptr ，否则 json.Unmarshal 会导致丢失具体类型
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, a)
	}
	{
		a := reflect.New(reflect.TypeOf([]int{})).Elem().Interface() // 这里 a 是 []int 类型
		t.Logf("a: %T", a)
		err := json.Unmarshal([]byte(`[1,2,3]`), &a)
		assert.NoError(t, err)
		t.Logf("a: %T", a)
		// 注意这里类型变了，是因为如果是通过 any hold 的话，需要确保 hold 的不能是 not-ptr / nil-ptr ，否则 json.Unmarshal 会导致丢失具体类型
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, a)
	}
	{
		// 下面的例子虽然 a 也是 []int 类型，但是并没有通过 any hold ，所以不会丢失具体类型
		a := reflect.New(reflect.TypeOf([]int{})).Elem()
		err := json.Unmarshal([]byte(`[1,2,3]`), a.Addr().Interface())
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, a.Interface())
	}
	{
		// 这个例子和上面的类似，换了种写法而已
		slicePtr := reflect.New(reflect.TypeOf([]int{}))
		err := json.Unmarshal([]byte(`[1,2,3]`), slicePtr.Interface())
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, reflect.Indirect(slicePtr).Interface())
	}
	{
		// 这里例子虽然 any hold 了，但是直接 hold 的是 *[]int 类型，所以不会丢失具体类型
		slicePtrValue := reflect.New(reflect.TypeOf([]int{}))
		slicePtr := slicePtrValue.Interface()
		err := json.Unmarshal([]byte(`[1,2,3]`), slicePtr)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, slicePtrValue.Elem().Interface())

		err = json.Unmarshal([]byte(`[2,3,4]`), &slicePtr) // 多次取地址也 OK
		assert.NoError(t, err)
		assert.Equal(t, []int{2, 3, 4}, slicePtrValue.Elem().Interface())
	}
}

func TestNil(t *testing.T) {
	{
		var a *int
		assert.True(t, reflect.ValueOf(a).IsNil())
	}
	{
		var a **int
		assert.True(t, reflect.ValueOf(a).IsNil())
	}
	{
		// IMPORTANT: 注意这里传入的是取了地址的
		// 可以理解为给空指针取地址得到的变量已经不是 nil 了，只是它的值是一个 nil 的指针，它不是 nil
		// 传入的是它，自然而然不是 nil
		var a *int
		assert.False(t, reflect.ValueOf(&a).IsNil())
	}
	{
		var a map[string]int
		assert.True(t, reflect.ValueOf(a).IsNil())

		b := reflect.New(reflect.TypeOf(map[string]int{})).Elem().Interface()
		assert.True(t, reflect.ValueOf(b).IsNil())

		c := reflect.New(reflect.TypeOf(int(0))).Elem().Interface()
		assert.Panics(t, func() {
			_ = reflect.ValueOf(c).IsNil()
		})

		d := reflect.New(reflect.TypeOf(lo.ToPtr(0))).Elem().Interface()
		assert.True(t, reflect.ValueOf(d).IsNil())
		// 综上所述，New 一个非标量类型，得到的值会是 nil
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
		// If v is a nil value with no type, unmarshal into a map[string]any.
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
		err := json.Unmarshal(data, &p)
		assert.NoError(t, err)
		assert.Equal(t, Person{Name: "Alice", Age: 30}, p)

		var q any = Person{}
		err = json.Unmarshal(data, &q)
		assert.NoError(t, err)
		// 这里 q 的类型是 map[string]any ，而不是 Person ，这就是直接使用 json.Unmarshal 的局限性
		assert.Equal(t, map[string]any{"age": float64(30), "name": "Alice"}, q)
	}

	{
		// 传入一个值，返回一个新的值
		data := []byte(`{"name":"Alice","age":30}`)
		var p Person
		newP, err := UnmarshalToNew(data, p)
		assert.NoError(t, err)
		assert.Equal(t, Person{Name: "Alice", Age: 30}, newP)
		assert.NotEqual(t, p, newP)

		var q any = Person{}
		newQ, err := UnmarshalToNew(data, q)
		assert.NoError(t, err)
		// 这里 newQ 的类型是 Person ，而不是 map[string]any ，这就是 UnmarshalToNew 的优点
		assert.Equal(t, Person{Name: "Alice", Age: 30}, newQ)
		assert.NotEqual(t, q, newQ)

		var qs any = []Person{}
		newQs, err := UnmarshalToNew([]byte(`[{"name":"Alice","age":30}]`), qs)
		assert.NoError(t, err)
		assert.Equal(t, []Person{{Name: "Alice", Age: 30}}, newQs)
		assert.NotEqual(t, qs, newQs)

		{
			var qs any = reflect.MakeSlice(reflect.TypeOf([]Person{}), 0, 0).Interface()
			newQs, err := UnmarshalToNew([]byte(`[{"name":"Alice","age":30}]`), qs)
			assert.NoError(t, err)
			assert.Equal(t, []Person{{Name: "Alice", Age: 30}}, newQs)
			assert.NotEqual(t, qs, newQs)
		}
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
		assert.Equal(t, map[string]any{"age": float64(30), "name": "Alice"}, newP)
		assert.NotEqual(t, p, newP)
	}
}

// 偶尔这个方法会比较方便，匹配 IsNil 的情况
func CanNil(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return true
	default:
		return false
	}
}
