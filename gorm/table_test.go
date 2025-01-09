package gorm

import (
	"cmp"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 目标是调研如何在 GORM 中实现动态表前缀的功能

type Foo struct {
	ID   string
	Name string
}

type Bar struct {
	ID   string
	Name string
}

func resetDBForTableTest() {
	db.Exec("DROP TABLE IF EXISTS foos")
	db.Exec("DROP TABLE IF EXISTS bars")
}

func TestNamingStrategyTablePrefix(t *testing.T) {
	resetDBForTableTest()
	require.NoError(t, db.AutoMigrate(&Foo{}))

	require.NoError(t, db.Create(&Foo{ID: "1", Name: "foo"}).Error)
	{
		foo := &Foo{}
		require.NoError(t, db.Where("id = ?", "1").First(foo).Error)
		require.Equal(t, "foo", foo.Name)
	}

	require.NoError(t, db.Exec(`CREATE SCHEMA IF NOT EXISTS plant;`).Error)

	db := db.Session(&gorm.Session{})
	db.Config.NamingStrategy = schema.NamingStrategy{
		TablePrefix:         "plant.",
		IdentifierMaxLength: 64,
	}
	{
		foo := &Foo{}
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id = ?", "1").First(foo)
		})
		require.NotContains(t, sql, "plant") // 因为 gorm.DB 对象在开始的时候已经因为 AutoMigrate 的执行缓存了 Foo 的相关信息，此时不会认作是 plant.foos
	}
	{
		require.NoError(t, db.AutoMigrate(&Bar{}))
		require.NoError(t, db.Create(&Bar{ID: "1", Name: "bar"}).Error)

		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Create(&Bar{ID: "1", Name: "bar"})
		})
		require.Contains(t, sql, "plant") // 因为在指定 plant. 前缀的时候还没有缓存 Bar 的相关信息，所以会认作是 plant.bars
	}
	// 所以这不是一个可用方案
}

func TestTable(t *testing.T) {
	resetDBForTableTest()
	require.NoError(t, db.AutoMigrate(&Foo{}))

	foo := &Foo{}
	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Table("plantx.foos").Where("id = ?", "1").First(foo)
	}), "plantx")
	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Table("planty.foos").Where("id = ?", "1").First(foo)
	}), "planty")

	require.NoError(t, db.Exec(`CREATE SCHEMA IF NOT EXISTS plant;`).Error)
	db := db.Table("plant.foos").Session(&gorm.Session{}) // Fixed TableName
	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("id = ?", "1").First(foo)
	}), "plant")

	// 明确指定了表名，当然可以，符合预期，但是不方便
}

func TestScopes(t *testing.T) {
	resetDBForTableTest()
	require.NoError(t, db.AutoMigrate(&Foo{}))

	callCount := 0
	scopeTableName := func(tableName string) func(*gorm.DB) *gorm.DB {
		return func(db *gorm.DB) *gorm.DB {
			callCount++
			return db.Table(tableName)
		}
	}
	{
		// 默认基本可以理解为 Scopes 方法是一次性的，用完即丢
		db := db.Scopes(scopeTableName("foos"))
		require.NoError(t, db.Create(&Foo{ID: "1", Name: "foo1"}).Error)
		require.NoError(t, db.Create(&Foo{ID: "2", Name: "foo2"}).Error)
		require.Equal(t, 1, callCount)
	}
	{
		// 如果通过 Session 来固定前面的 Scopes 方法，那么就可以多次使用
		db := db.Scopes(scopeTableName("foos")).Session(&gorm.Session{}) // fixed
		require.NoError(t, db.Create(&Foo{ID: "3", Name: "foo1"}).Error)
		require.NoError(t, db.Create(&Foo{ID: "4", Name: "foo2"}).Error)
		require.Equal(t, 1+2, callCount)
	}

	// 所以这个方案是可行的，但是实际执行上需要其他信息和兼容其他情况，参考 TestDynamicTablePrefix
}

func ParseSchemaWithDB(db *gorm.DB, v any) (*schema.Schema, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(v); err != nil {
		return nil, errors.Wrap(err, "parse schema with db")
	}
	return stmt.Schema, nil
}

const dbKeyTablePrefix = "__table_prefix__"

// TablePrefix dynamic table prefix
// Only scenarios where a Model is provided are supported
func TablePrefix(tablePrefix string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if v, ok := db.Get(dbKeyTablePrefix); ok {
			if v.(string) != tablePrefix {
				panic(fmt.Sprintf("table prefix is already set to %s", v))
			}
			return db
		}

		if tablePrefix == "" {
			return db
		}

		stmt := db.Statement
		model := cmp.Or(stmt.Model, stmt.Dest)
		if model == nil {
			return db
		}

		var table string
		if stmt.Table != "" {
			table = stmt.Table
		} else {
			s, err := ParseSchemaWithDB(db, model)
			if err != nil {
				db.AddError(err)
				return db
			}
			table = s.Table
		}
		return db.Set(dbKeyTablePrefix, tablePrefix).Table(tablePrefix + table)
	}
}

func TestTablePrefix(t *testing.T) {
	resetDBForTableTest()

	getTableName := func(db *gorm.DB, tablePrefix string, model any) string {
		s, _ := ParseSchemaWithDB(db, model)
		return tablePrefix + s.Table
	}

	type Foox struct {
		ID   string
		Name string
	}

	type Barx struct {
		ID   string
		Name string
	}

	prefix := "some_"

	db := db.Scopes(TablePrefix(prefix)).Session(&gorm.Session{})
	require.NoError(t, db.AutoMigrate(&Foox{}, &Barx{}))

	foo := &Foox{}
	require.Equal(t, "some_fooxes", getTableName(db, prefix, foo))
	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("id = ?", "1").First(foo)
	}), "some_fooxes")

	bar := &Barx{}
	require.Equal(t, "some_barxes", getTableName(db, prefix, bar))
	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("id = ?", "1").First(bar)
	}), "some_barxes")

	require.Contains(t, db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Table("x_barxes").Where("id = ?", "1").First(bar)
	}), "some_x_barxes")
}
