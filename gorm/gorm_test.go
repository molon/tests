package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theplant/testenv"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func TestMain(m *testing.M) {
	env, err := testenv.New().DBEnable(true).SetUp()
	if err != nil {
		panic(err)
	}
	defer env.TearDown()

	db = env.DB
	db.Logger = db.Logger.LogMode(logger.Info)

	if err = db.AutoMigrate(&KV{}, &User{}, &Address{}); err != nil {
		panic(err)
	}

	m.Run()
}

type KV struct {
	Key   string `json:"key" gorm:"primaryKey;not null;"`
	Value string `json:"value" gorm:"not null;"`
}

func TestOr(t *testing.T) {
	var err error
	err = db.Create(&KV{Key: "k1", Value: "v1"}).Error
	require.Nil(t, err)
	err = db.Create(&KV{Key: "k2", Value: "v2"}).Error
	require.Nil(t, err)

	{
		kvs := []*KV{}
		// SELECT * FROM "kvs" WHERE key = 'k1' OR key = 'k2' ORDER BY key DESC
		err = db.Where("key = ?", "k1").Or("key = ?", "k2").Order("key DESC").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}
	{
		kvs := []*KV{}
		// SELECT * FROM "kvs" WHERE key = 'k1' OR key = 'k2' ORDER BY key DESC
		err = db.Or("key = ?", "k2").Where("key = ?", "k1").Order("key DESC").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}
	// 所以 Or 和 Where 是平级的，且不会有优先级问题，但是依然不建议这么写，容易引起误解

	{
		kvs := []*KV{}
		// SELECT * FROM "kvs" WHERE key = 'k1' OR key = 'k2' AND key IS NOT NULL
		err = db.Or("key = ?", "k2").Where("key = ?", "k1").Where("key IS NOT NULL").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}

	{
		kvs := []*KV{}
		// SELECT * FROM "kvs" WHERE key IS NOT NULL OR key = 'k2' AND key = 'k1'
		err = db.Where("key IS NOT NULL").Or("key = ?", "k2").Where("key = ?", "k1").Find(&kvs).Error
		require.Nil(t, err)
		require.Len(t, kvs, 2)
	}

	// TIPS: 但是后续发现如果 Model 里存在 DeletedAt 的话，行为会和上面的不一致，注意不要被这个坑到，之后再补充测试，
}

type User struct {
	gorm.Model
	Name      string
	Addresses []Address
}

type Address struct {
	gorm.Model
	AddressLine string
	UserID      uint
	User        User
}

func TestAssociation(t *testing.T) {
	{
		user := User{
			Name: "Alice",
			Addresses: []Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Create(&user).Error) // 会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)

		t.Logf("User: %+v", user)
		user.Addresses[0].AddressLine = "789 Boulevard"
		firstAddress := user.Addresses[0]
		require.NoError(t, db.Updates(user).Error) // 不会进行关联更新
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)
		require.NoError(t, db.Updates(firstAddress).Error)
		require.NoError(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error)

		require.NoError(t, db.Delete(&user).Error) // 不会进行关联删除
		require.NoError(t, db.First(&user.Addresses).Error)

		db.Exec("TRUNCATE TABLE users")
		db.Exec("TRUNCATE TABLE addresses")
	}

	// with Omit(clause.Associations)
	{
		user := User{
			Name: "Alice",
			Addresses: []Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Omit(clause.Associations).Create(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		db.Exec("TRUNCATE TABLE users")
		db.Exec("TRUNCATE TABLE addresses")
	}

	{
		user := User{
			Name: "Alice",
			Addresses: []Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)

		db.Exec("TRUNCATE TABLE users")
		db.Exec("TRUNCATE TABLE addresses")
	}

	// with Omit(clause.Associations)
	{
		user := User{
			Name: "Alice",
			Addresses: []Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Omit(clause.Associations).Save(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		db.Exec("TRUNCATE TABLE users")
		db.Exec("TRUNCATE TABLE addresses")
	}
}
