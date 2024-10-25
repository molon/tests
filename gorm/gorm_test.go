package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theplant/testenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type KV struct {
	Key   string `json:"key" gorm:"primaryKey;not null;"`
	Value string `json:"value" gorm:"not null;"`
}

func TestMain(m *testing.M) {
	env, err := testenv.New().DBEnable(true).SetUp()
	if err != nil {
		panic(err)
	}
	defer env.TearDown()

	db = env.DB
	db.Logger = db.Logger.LogMode(logger.Info)

	if err = db.AutoMigrate(&KV{}); err != nil {
		panic(err)
	}

	m.Run()
}

func TestOr(t *testing.T) {
	var err error
	err = db.Create(&KV{Key: "k1", Value: "v1"}).Error
	require.Nil(t, err)
	err = db.Create(&KV{Key: "k2", Value: "v2"}).Error
	require.Nil(t, err)

	{
		kvs := []*KV{}
		err = db.Where("key = ?", "k1").Or("key = ?", "k2").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}
	{
		kvs := []*KV{}
		err = db.Or("key = ?", "k2").Where("key = ?", "k1").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}
	// 所以 Or 和 Where 是平级的，且不会有优先级问题
}
