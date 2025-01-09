package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type Table1 struct {
	ID   string `gorm:"index;not null;"`
	Name string
}

func (*Table1) TableName() string {
	return "tmp_table"
}

type Table2 struct {
	ID    string `gorm:"index;not null;"`
	Name  string
	Scope string `gorm:"index;"`
}

func (*Table2) TableName() string {
	return "tmp_table"
}

type Table3 struct {
	ID          string `gorm:"index;not null;"`
	Name        string
	Scope       string `gorm:"index;"`
	Description string `gorm:"not null;"`
}

func (*Table3) TableName() string {
	return "tmp_table"
}

func TestAutoMigrateAfterAddNullableField(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&Table1{}))

	db.Create(&Table1{ID: "1", Name: "foo"})

	require.NoError(t, db.AutoMigrate(&Table2{}))

	m := &Table2{}
	require.NoError(t, db.First(m, "id = ?", "1").Error)
	require.Equal(t, "foo", m.Name)
	require.Equal(t, "", m.Scope)

	require.ErrorContains(t, db.AutoMigrate(&Table3{}), "contains null values")
}
