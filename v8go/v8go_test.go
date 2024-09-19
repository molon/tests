package v8go

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"rogchap.com/v8go"
)

// 推荐先去看 goja_test.go
// 这里的测试可对比 v8go 和 goja 的不同行为

// 文档相关
// TerminateExecution: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#ab33b5ea0fbd412614931733449c3d659
// CancelTerminateExecution: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#accd54de1bf8bdb27a541a578241d4870
// IsExecutionTerminating: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#adb638de648962913ddaba7d75348da27
// 以上时和 TerminateExecution 相关的方法，经过调研得出结论
//  1. 在 js 执行过程中需要回调 go 逻辑时，才需要在关键位置通过 IsExecutionTerminating 来判断是否需要 return
//  2. CancelTerminateExecution 这个 v8go 压根就没有暴露，通常我们也用不到，它是在 TerminateExecution 发出后和脚本将异常传递出来之间的这段时间才有意义，
//     即使暴露，也是需要在 js 执行过程中才需要用到做非常细粒度的控制，不太需要关心
func TestTerminateExecution(t *testing.T) {
	const SCRIPT = `
	var i = 0;
	for (;;) {
		i++;
	}
	`

	vm := v8go.NewContext()

	time.AfterFunc(100*time.Millisecond, func() {
		// 可以在另外一个线程中调用
		vm.Isolate().TerminateExecution()
	})
	val, err := vm.RunScript(SCRIPT, "")
	assert.Nil(t, val)
	assert.ErrorContains(t, err, "ExecutionTerminated")

	{
		// 可以看到即使上面的被中断了，下面的 vm.RunScript() 仍然可以正常执行
		val, err := vm.RunScript(`"hello world"`, "")
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	vm.Isolate().TerminateExecution()

	{
		// 可以看到即使上面执行了 TerminateExecution ，下面的 vm.RunScript() 仍然可以正常执行
		// IMPORTANT: 所以这个和 goja 的行为是不一样的，goja 的行为是在无脚本执行时也可标记中断，然后其会影响下一次的脚本执行
		val, err := vm.RunScript(`"hello world"`, "")
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}
}

// 结合上述特点，封装一个方法来实现
func WrapRun(
	ctx context.Context,
	v8ctx *v8go.Context,
	f func(v8ctx *v8go.Context) (*v8go.Value, error),
) (result *v8go.Value, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context already done")
	}

	stop := context.AfterFunc(ctx, func() {
		v8ctx.Isolate().TerminateExecution()
	})
	defer stop()

	result, err = f(v8ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.Wrap(ctx.Err(), "failed to run")
		}
		return nil, errors.Wrap(err, "failed to run")
	}
	return result, nil
}

func TestWrapRun(t *testing.T) {
	const SCRIPT = `
	var i = 0;
	for (;;) {
		i++;
	}
	`

	vm := v8go.NewContext()
	{
		// 可以看到 ctx 超时按预期工作
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		val, err := WrapRun(ctx, vm, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript(SCRIPT, "")
		})
		assert.Nil(t, val)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		since := time.Since(start)
		assert.GreaterOrEqual(t, since, 200*time.Millisecond)
		assert.Less(t, since, 205*time.Millisecond)
	}

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, err := WrapRun(ctx, vm, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript("1 + 2", "")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
	}

	vm.Isolate().TerminateExecution()

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, err := WrapRun(ctx, vm, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript("1 + 2", "")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
	}
}