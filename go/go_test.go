package std

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
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

	func(v any) {
		require.True(t, v == nil)
	}(nil)

	var nilFoo *errFoo
	func(v any) {
		require.False(t, v == nil) // any hold not nil
		require.True(t, reflect.ValueOf(v).IsNil())
	}(nilFoo)
	func(v any) {
		require.False(t, v == nil)                   // any hold not nil
		require.False(t, reflect.ValueOf(v).IsNil()) // nil 指针的指针 不算空指针
	}(&nilFoo)

	// 综上，对于一个 interface 类型变量来说，只要其值有具体类型，其就不是 nil
	// 所以特别注意如果一个方法对调用方而言，其期望的通常是 interface 类型已经足够，并且有可能为 nil 的时候，就不要返回具体类型
	// 最典型的就是 error 类型，通常应该返回 error 类型，而不是具体类型，否则对调用方而言会难以处理
}

// TestNamedReturnCrash 是专门用于子进程执行的测试
// 它会导致无限递归和栈溢出崩溃
func TestNamedReturnCrash(t *testing.T) {
	// 只在环境变量指定的子进程中运行
	if os.Getenv("TEST_RECURSIVE_CRASH") != "1" {
		t.Skip("跳过此测试，仅在子进程模式下运行")
		return
	}

	createFuncWithNamedReturn := func() (f func() int) {
		// f 已被初始化为零值(nil)

		// 给返回值 f 赋值
		f = func() int { return 1 }

		// 创建闭包并返回，闭包引用了外部的f变量(命名返回值)
		return func() int {
			// 此处调用f()，而f实际上是当前函数自身，形成递归无限调用
			return f() + 1
		}
	}

	fn := createFuncWithNamedReturn()
	fn() // 触发无限递归和栈溢出
}

func TestNamedReturnInfiniteRecursion(t *testing.T) {
	// 如果是子进程模式，则执行递归崩溃测试
	if os.Getenv("TEST_RECURSIVE_CRASH") == "1" {
		createFuncWithNamedReturn := func() (f func() int) {
			// 给返回值 f 赋值
			f = func() int { return 1 }

			// 返回闭包，闭包引用了并且修改了命名返回值 f ，形成无限递归
			return func() int {
				return f() + 1
			}
		}

		fn := createFuncWithNamedReturn()
		fn() // 触发无限递归和栈溢出
		return
	}

	// 主测试：启动子进程执行递归崩溃测试
	cmd := exec.Command(os.Args[0],
		"-test.run=TestNamedReturnInfiniteRecursion")
	cmd.Env = append(os.Environ(), "TEST_RECURSIVE_CRASH=1")

	output, err := cmd.CombinedOutput()

	require.Error(t, err, "期望子进程因栈溢出而失败")

	t.Logf("子进程输出: %s", strings.Split(string(output), "\n")[0])
	t.Logf("子进程错误: %v", err)

	// 安全的实现作为对照
	createSafeFunc := func() func() int {
		f := func() int { return 1 }
		return func() int {
			return f() + 1
		}
	}

	safeFunc := createSafeFunc()
	result := safeFunc()

	require.Equal(t, 2, result)

	t.Log("命名返回值陷阱：当返回的闭包引用了命名返回值自身时会导致无限递归和栈溢出")
}
