package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 定义测试用的模型
type TestBar struct {
	ProductCode uint
	Name        string
}

// TestFoo 与 TestBar 是 belongsTo 关系
type TestFoo struct {
	ID      uint
	Name    string
	BarCode uint
}

func (*TestFoo) TableName() string {
	return "test_foos"
}

// TestFooWrapper 嵌入 TestFoo 并添加新的 Bar 关系
type TestFooWrapper struct {
	*TestFoo          // 嵌入 TestFoo 复用其表结构和 BarCode 字段
	Bar      *TestBar `gorm:"foreignKey:BarCode;references:ProductCode"` // 使用 TestFoo 中的 BarCode
}

func TestJoins(t *testing.T) {
	// 清理并准备数据库
	require.NoError(t, db.Migrator().DropTable(&TestFooWrapper{}, &TestFoo{}, &TestBar{}))
	require.NoError(t, db.AutoMigrate(&TestBar{}, &TestFoo{}, &TestFooWrapper{}))

	// 插入测试数据
	bar1 := TestBar{ProductCode: 101, Name: "Bar1"}
	bar2 := TestBar{ProductCode: 102, Name: "Bar2"}
	require.NoError(t, db.Create(&bar1).Error)
	require.NoError(t, db.Create(&bar2).Error)

	// 修改为使用 BarCode 而不是 BarID
	foo1 := TestFoo{Name: "Foo1", BarCode: bar1.ProductCode}
	foo2 := TestFoo{Name: "Foo2", BarCode: bar2.ProductCode}
	require.NoError(t, db.Create(&foo1).Error)
	require.NoError(t, db.Create(&foo2).Error)

	// 测试 TestFooWrapper 和 TestBar 的 Joins
	t.Run("TestFooWrapperBarJoins", func(t *testing.T) {
		var results []TestFooWrapper

		// 使用 Joins 测试 Foo 与 Bar 的关联
		err := db.Joins("Bar").Find(&results).Error
		require.NoError(t, err)
		require.Len(t, results, 2)

		// 验证查询结果
		require.Equal(t, "Foo1", results[0].Name)
		require.Equal(t, "Bar1", results[0].Bar.Name)
		require.Equal(t, "Foo2", results[1].Name)
		require.Equal(t, "Bar2", results[1].Bar.Name)

		// 测试使用 Preload 加载 Bar
		var wrapper TestFooWrapper
		err = db.Preload("Bar").First(&wrapper).Error
		require.NoError(t, err)
		require.Equal(t, "Foo1", wrapper.Name)
		require.NotNil(t, wrapper.Bar)
		require.Equal(t, "Bar1", wrapper.Bar.Name)
	})
}
