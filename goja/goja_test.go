package goja

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestInterrupt(t *testing.T) {
	const SCRIPT = `
	var i = 0;
	for (;;) {
		i++;
	}
	`

	vm := goja.New()
	time.AfterFunc(100*time.Millisecond, func() {
		vm.Interrupt("halt") // 可以在另外一个线程中调用
	})
	val, err := vm.RunString(SCRIPT)
	assert.Nil(t, val)
	assert.ErrorContains(t, err, "halt")
	assert.ErrorAs(t, err, lo.ToPtr(new(goja.InterruptedError)))

	{
		// 可以看到即使上面的 vm.RunString() 被中断了，下面的 vm.RunString() 仍然可以正常执行
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	vm.Interrupt("halt2")

	{
		// 可以发现此时 vm 已经无法正常工作了
		// IMPORTANT: 这是因为如果 Interrupt 是在脚本运行过程中调用，确实执行了中断效果，此时其 Interrupted 标记会被重置回去
		// 而如果 Interrupt 是在脚本运行之前调用，那么并没有脚本被真的中断了，其 Interrupted 标记就不会被重置回去，此时 vm 就无法正常工作了
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, val)
		assert.ErrorContains(t, err, "halt2")
		assert.ErrorAs(t, err, lo.ToPtr(new(goja.InterruptedError)))
	}

	{
		// 可以发现现在又正常工作了，这是因为上一个 Interrupted 标记确实执行了中断效果后被重置回去了
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	vm.Interrupt("halt3")

	{
		vm.ClearInterrupt()
		// 可以发现正常工作，这是因为我们主动 clear 了 Interrupted 标记
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	{
		vm.ClearInterrupt()
		// 可以发现即使前面没有 Interrupt 调用，执行 ClearInterrupt 也不会有什么副作用
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	{
		// 可以发现在 ClearInterrupt 之后再次调用 Interrupt 也肯定不行
		vm.ClearInterrupt()
		vm.Interrupt("halt4")
		val, err := vm.RunString(`"hello world"`)
		assert.Nil(t, val)
		assert.ErrorContains(t, err, "halt4")
		assert.ErrorAs(t, err, lo.ToPtr(new(goja.InterruptedError)))
	}
}

// 结合上述特点，封装一个方法来实现
func WrapRun(
	ctx context.Context,
	runtime *goja.Runtime,
	f func(runtime *goja.Runtime) (goja.Value, error),
) (result goja.Value, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context already done")
	}

	// 直接在 f 运行之前保证之前的 Interrupted 标记被清除，此方法内部其实只是执行了一个 atomic.Store 所以成本其实很低
	runtime.ClearInterrupt()

	stop := context.AfterFunc(ctx, func() {
		runtime.Interrupt("halt")
	})
	defer stop()

	result, err = f(runtime)
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

	vm := goja.New()
	{
		// 可以看到 ctx 超时按预期工作
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		val, err := WrapRun(ctx, vm, func(runtime *goja.Runtime) (goja.Value, error) {
			return runtime.RunString(SCRIPT)
		})
		assert.Nil(t, val)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		since := time.Since(start)
		assert.GreaterOrEqual(t, since, 100*time.Millisecond)
		assert.Less(t, since, 105*time.Millisecond)
	}

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, err := WrapRun(ctx, vm, func(runtime *goja.Runtime) (goja.Value, error) {
			return runtime.RunString("1 + 2")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
	}

	vm.Interrupt("halt")

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, err := WrapRun(ctx, vm, func(runtime *goja.Runtime) (goja.Value, error) {
			return runtime.RunString("1 + 2")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
	}
}
