package std

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type errFoo struct{}

func (*errFoo) Error() string {
	return "errFoo"
}

func TestNil(t *testing.T) {
	var a any
	require.True(t, a == nil)

	a = (*time.Time)(nil)
	require.True(t, a != nil)

	a = func() *time.Time { return nil }()
	require.True(t, a != nil)

	b := func() *time.Time { return nil }()
	require.True(t, b == nil)

	a = b
	require.True(t, a != nil)

	a = func() any { return nil }()
	require.True(t, a == nil)

	a = func() any { return (*time.Time)(nil) }()
	require.True(t, a != nil)

	var e error
	require.True(t, e == nil)

	e = func() error { return nil }()
	require.True(t, e == nil)

	e = func() *errFoo { return nil }()
	require.True(t, e != nil)
	require.True(t, lo.IsNil(e))

	// 综上，对于一个 interface 类型变量来说，只要其值有具体类型，其就不是 nil
	// 所以特别注意如果一个方法对调用方而言，其期望的通常是 interface 类型已经足够，并且有可能为 nil 的时候，就不要返回具体类型
	// 最典型的就是 error 类型，通常应该返回 error 类型，而不是具体类型，否则对调用方而言会难以处理
}
