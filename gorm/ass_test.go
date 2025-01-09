package gorm

import (
	"testing"
	"time"

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
	db.Config.DisableForeignKeyConstraintWhenMigrating = true

	m.Run()
}

type KV struct {
	Key   string `json:"key" gorm:"primaryKey;not null;"`
	Value string `json:"value" gorm:"not null;"`
}

func TestOr(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&KV{}))

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
	Model
	Name      string
	Addresses []*Address
}

type Address struct {
	Model
	AddressLine string
	UserID      uint
	User        User
}

func TestAssociation(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Create(&user).Error) // 会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)

		// t.Logf("User: %+v", user)
		user.Addresses[0].AddressLine = "789 Boulevard"
		firstAddress := user.Addresses[0]
		require.NoError(t, db.Updates(&user).Error) // 不会进行关联更新
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)
		require.NoError(t, db.Updates(&firstAddress).Error)
		require.NoError(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error)

		require.NoError(t, db.Delete(&user).Error) // 不会进行关联删除
		require.NoError(t, db.First(&user.Addresses).Error)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// with Omit(clause.Associations)
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Omit(clause.Associations).Create(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)

		user.Addresses[0].AddressLine = "789 Boulevard"
		firstAddress := user.Addresses[0]
		require.NoError(t, db.Save(&user).Error) // 不会进行关联更新
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		user.Addresses = []*Address{firstAddress}
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Save(&user).Error) // 会进行关联更新
		require.NoError(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error)
		require.NoError(t, db.Where("address_line = ?", "456 Avenue").First(&user.Addresses).Error) // 但是另外一个不会被删除，一些场景下就很沙雕
		addresses := []*Address{}
		require.NoError(t, db.Where("user_id = ?", user.ID).Find(&addresses).Error)
		require.Len(t, addresses, 2) // 但是另外一个不会被删除，一些场景下就很沙雕，几乎没法用这个破玩意

		// 不会进行关联更新，还是 .Omit(clause.Associations) 的优先级会更高，这很好
		firstAddress.AddressLine = "666 Boulevard"
		user.Addresses = []*Address{firstAddress}
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Omit(clause.Associations).Save(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// with Omit(clause.Associations)
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Omit(clause.Associations).Save(&user).Error)
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 测试关联删除
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error)

		require.NoError(t, db.Select(clause.Associations).Delete(&user).Error) // 会进行关联删除
		require.ErrorIs(t, db.First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}
	// with Omit(clause.Associations)
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error)

		require.NoError(t, db.Select(clause.Associations).Omit(clause.Associations).Delete(&user).Error) // 不会进行关联删除，后者优先级更高
		require.NoError(t, db.First(&user.Addresses).Error)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}
	// with Omit(clause.Associations) twice
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error)

		// 不会进行关联删除，后者优先级更高，多次 Omit 也一样
		require.NoError(t, db.Select(clause.Associations).Omit(clause.Associations).Omit(clause.Associations).Delete(&user).Error)
		require.NoError(t, db.First(&user.Addresses).Error)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 综上所述，Create 和 Save 非必要最好添加 .Omit(clause.Associations) ，以免非预期的语句执行，下面会更便捷的法子
}

func afterHandleWithoutAssociation(t *testing.T, db *gorm.DB, checkSkippedHooksMethod bool) {
	// 移除之后，所有的关联写操作
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Create(&user).Error) // 不会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		user.Addresses = []*Address{
			{AddressLine: "123 Street", UserID: user.ID},
			{AddressLine: "456 Avenue", UserID: user.ID},
		}
		require.NoError(t, db.Create(&user.Addresses).Error) // 主动创建一下

		// t.Logf("User: %+v", user)
		user.Name = "Bob"
		user.Addresses[0].AddressLine = "789 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&user).Error) // 不会进行关联更新
		require.NoError(t, db.Where("name = ?", "Bob").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Where("user_id = ?", user.ID).Find(&(user.Addresses)).Error) // 先查询出来
		require.Len(t, user.Addresses, 2)

		if checkSkippedHooksMethod {
			user.Addresses[0].AddressLine = "666 Boulevard"
			require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Model(&user).UpdateColumn("name", "BobX").Error) // 不会进行关联更新
			require.NoError(t, db.Where("name = ?", "BobX").First(&user).Error)
			require.ErrorIs(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)
		}

		require.NoError(t, db.Select(clause.Associations).Delete(&user).Error) // 不会进行关联删除
		require.NoError(t, db.First(&user.Addresses).Error)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 如果也显式指定了 Omit
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Omit(clause.Associations).Create(&user).Error) // 不会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		user.Addresses = []*Address{
			{AddressLine: "123 Street", UserID: user.ID},
			{AddressLine: "456 Avenue", UserID: user.ID},
		}
		require.NoError(t, db.Omit(clause.Associations).Create(&user.Addresses).Error) // 主动创建一下

		// t.Logf("User: %+v", user)
		user.Name = "Bob"
		user.Addresses[0].AddressLine = "789 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Omit(clause.Associations).Updates(&user).Error) // 不会进行关联更新
		require.NoError(t, db.Where("name = ?", "Bob").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Where("user_id = ?", user.ID).Find(&(user.Addresses)).Error) // 先查询出来
		require.Len(t, user.Addresses, 2)

		// if checkSkippedHooks {
		user.Addresses[0].AddressLine = "666 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Model(&user).Omit(clause.Associations).UpdateColumn("name", "BobX").Error) // 不会进行关联更新
		require.NoError(t, db.Where("name = ?", "BobX").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)
		// }

		require.NoError(t, db.Select(clause.Associations).Omit(clause.Associations).Delete(&user).Error) // 不会进行关联删除
		require.NoError(t, db.First(&user.Addresses).Error)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// Preload 操作依然可用
	{
		user := &User{
			Name: "Alice",
		}
		require.NoError(t, db.Create(user).Error)
		require.NoError(t, db.Create(&[]*Address{
			{AddressLine: "123 Street", UserID: user.ID},
			{AddressLine: "456 Avenue", UserID: user.ID},
		}).Error)

		require.NoError(t, db.Preload("Addresses").First(&user).Error)
		require.Len(t, user.Addresses, 2)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}
}

// 测试全局移除关联数据的写行为
func TestWithoutAssociationByRemoveCallbacks(t *testing.T) {
	env, err := testenv.New().DBEnable(true).SetUp()
	if err != nil {
		panic(err)
	}
	defer env.TearDown()

	db := env.DB
	db.Logger = db.Logger.LogMode(logger.Info)
	db.Config.DisableForeignKeyConstraintWhenMigrating = true

	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	// 未移除之前，所有的关联写操作
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Create(&user).Error) // 会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error)

		// t.Logf("User: %+v", user)
		user.Addresses[0].AddressLine = "789 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&user).Error) // 会进行关联更新
		require.NoError(t, db.Where("address_line = ?", "789 Boulevard").First(&user.Addresses).Error)
		require.NoError(t, db.Where("address_line = ?", "456 Avenue").First(&user.Addresses).Error) // 另外一个也不会被删除，一些场景下就很沙雕

		user.Addresses[0].AddressLine = "666 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Model(&user).UpdateColumn("name", "Bob").Error) // 会进行关联更新
		require.NoError(t, db.Where("name = ?", "Bob").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error)

		require.NoError(t, db.Select(clause.Associations).Delete(&user).Error)      // 会进行关联删除
		require.ErrorIs(t, db.First(&user.Addresses).Error, gorm.ErrRecordNotFound) // 倒是会全部删除

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 注意这个移除是 gorm.DB 级别的，不是 gorm.Session 级别的
	createCallback := db.Callback().Create()
	createCallback.Remove("gorm:save_before_associations")
	createCallback.Remove("gorm:save_after_associations")

	deleteCallback := db.Callback().Delete()
	deleteCallback.Remove("gorm:delete_before_associations")

	updateCallback := db.Callback().Update()
	updateCallback.Remove("gorm:save_before_associations")
	updateCallback.Remove("gorm:save_after_associations")

	// 没准这种方式更完整，是否会有隐患呢？
	afterHandleWithoutAssociation(t, db, true)
}

// 测试根据 hooks 来进行移除关联数据的写行为
var withoutAssociationByHooks = false

type Model struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (u *Model) BeforeCreate(tx *gorm.DB) error {
	if withoutAssociationByHooks {
		tx.Statement.Omit(clause.Associations)
	}
	return nil
}

func (u *Model) BeforeUpdate(tx *gorm.DB) (err error) {
	if withoutAssociationByHooks {
		tx.Statement.Omit(clause.Associations)
	}
	return nil
}

func (u *Model) BeforeDelete(tx *gorm.DB) (err error) {
	if withoutAssociationByHooks {
		tx.Statement.Omit(clause.Associations)
	}
	return nil
}

func TestWithoutAssociationByHooks(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	withoutAssociationByHooks = true
	defer func() {
		withoutAssociationByHooks = false
	}()

	// 这种方式会因为例如 UpdateColumn 之类的操作不会触发 Hooks 而导致处理得不够完整
	// 使用者避免使用 UpdateColumn 之类的方法+FullSaveAssociations 就能避免这个问题
	afterHandleWithoutAssociation(t, db, false)
}

func TestWithoutAssociationByScope(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	// 依靠一个 scope+session 来固定 Omit 语句
	db := db.Scopes(func(db *gorm.DB) *gorm.DB {
		return db.Omit(clause.Associations)
	}).Session(&gorm.Session{})

	// 本来以为这样的话 Preload 就不行了呢，没想到也行。
	// 而且本以为对 Delete 来说 .Omit(clause.Associations) 在前/.Select(clause.Associations) 在后 这种情况会认后者呢，没想到也不会
	// 所以貌似和感知上有点不匹配，没准会算是 gorm 以后可能会修复的 bug ？
	afterHandleWithoutAssociation(t, db, true)
}

type UserWithHook struct {
	gorm.Model
	Name string
	logs []string
}

func (u *UserWithHook) cleanLogs() {
	u.logs = []string{}
}

func (u *UserWithHook) BeforeSave(tx *gorm.DB) error {
	u.logs = append(u.logs, "BeforeSave")
	return nil
}

func (u *UserWithHook) BeforeCreate(tx *gorm.DB) error {
	u.logs = append(u.logs, "BeforeCreate")
	return nil
}

func (u *UserWithHook) BeforeUpdate(tx *gorm.DB) (err error) {
	u.logs = append(u.logs, "BeforeUpdate")
	return nil
}

func (u *UserWithHook) BeforeDelete(tx *gorm.DB) (err error) {
	u.logs = append(u.logs, "BeforeDelete")
	return nil
}

func TestHooks(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&UserWithHook{}))

	{
		user := UserWithHook{
			Name: "Alice",
		}
		// 执行 Create 时，也会执行 BeforeSave
		require.NoError(t, db.Create(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeCreate", user.logs[1])

		require.NoError(t, db.Exec("TRUNCATE TABLE user_with_hooks").Error)
	}

	{
		user := UserWithHook{
			Name: "Alice",
		}
		// 执行 Save 时，也会执行 BeforeCreate ，和 Create 没差
		require.NoError(t, db.Save(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeCreate", user.logs[1])

		user.cleanLogs()
		user.Name = "Bob"
		// 如果发现 Save 最终执行的是 Update 语句，那么 BeforeCreate 就不会执行，BeforeUpdate 会执行
		require.NoError(t, db.Save(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeUpdate", user.logs[1])

		user.cleanLogs()
		user.Name = "Charlie"
		require.NoError(t, db.Updates(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeUpdate", user.logs[1])

		user.cleanLogs()
		// UpdateColumn 不会触发 BeforeUpdate ，并且不会更新 UpdatedAt
		origUpdatedAt := user.UpdatedAt
		require.NoError(t, db.Model(&user).UpdateColumn("name", "David").Error)
		require.Len(t, user.logs, 0)                    // 不会触发 Hooks
		require.Equal(t, origUpdatedAt, user.UpdatedAt) // 不会更新 UpdatedAt
		require.Equal(t, user.Name, "David")            // 会修改字段

		user.cleanLogs()
		require.NoError(t, db.Model(&user).Updates(map[string]any{
			"name": "Eve",
		}).Error)
		require.Len(t, user.logs, 2)                       // 会触发 BeforeSave 和 BeforeUpdate
		require.NotEqual(t, origUpdatedAt, user.UpdatedAt) // 会更新 UpdatedAt
		require.Equal(t, user.Name, "Eve")                 // 会修改字段
		// t.Logf("UpdatedAt: %v", user.UpdatedAt)

		user.cleanLogs()
		require.NoError(t, db.Delete(&user).Error)
		require.Len(t, user.logs, 1)
		require.Equal(t, "BeforeDelete", user.logs[0])

		require.NoError(t, db.Exec("TRUNCATE TABLE user_with_hooks").Error)
	}

	// 综上所述
	// BeforeSave 在 Save/Create/Updates 时都会执行，BeforeCreate 只会在最终是需要 Create 时执行，BeforeUpdate 只会在最终是需要 Update 时执行
	// BeforeDelete 在 Delete 时才会执行
}
