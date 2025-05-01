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

// 目标是调研 gorm 默认的对关联数据的写行为，并且调研如何移除这个默认行为
type User struct {
	Model
	Name      string
	Age       int
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
		require.NoError(t, db.Omit(clause.Associations).Create(&user).Error) // 不会进行关联创建
		require.NoError(t, db.Where("name = ?", "Alice").First(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "123 Street").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 测试关联更新
	{
		user := User{
			Name: "Alice",
			Addresses: []*Address{
				{AddressLine: "123 Street"},
				{AddressLine: "456 Avenue"},
			},
		}
		require.NoError(t, db.Save(&user).Error) // 会进行关联创建
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
		require.NoError(t, db.Where("address_line = ?", "456 Avenue").First(&user.Addresses).Error) // 但是另外一个不会被删除
		addresses := []*Address{}
		require.NoError(t, db.Where("user_id = ?", user.ID).Find(&addresses).Error)
		require.Len(t, addresses, 2) // 但是另外一个不会被删除

		// 不会进行关联更新，还是 .Omit(clause.Associations) 的优先级会更高，这很好
		firstAddress.AddressLine = "666 Boulevard"
		user.Addresses = []*Address{firstAddress}
		require.NoError(t, db.Omit(clause.Associations).Session(&gorm.Session{FullSaveAssociations: true}).Save(&user).Error)
		require.ErrorIs(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error, gorm.ErrRecordNotFound)

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

		require.NoError(t, db.Omit(clause.Associations).Select(clause.Associations).Delete(&user).Error) // 不会进行关联删除，Omit 优先级更高
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

func afterHandleOmitAssociations(t *testing.T, db *gorm.DB, checkSkippedHooksMethod bool) {
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
func TestOmitAssociationsByRemoveCallbacks(t *testing.T) {
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
		require.NoError(t, db.Where("address_line = ?", "456 Avenue").First(&user.Addresses).Error) // 另外一个也不会被删除

		user.Addresses[0].AddressLine = "666 Boulevard"
		require.NoError(t, db.Session(&gorm.Session{FullSaveAssociations: true}).Model(&user).UpdateColumn("name", "Bob").Error) // 会进行关联更新
		require.NoError(t, db.Where("name = ?", "Bob").First(&user).Error)
		require.NoError(t, db.Where("address_line = ?", "666 Boulevard").First(&user.Addresses).Error)

		require.NoError(t, db.Select(clause.Associations).Delete(&user).Error)      // 会进行关联删除
		require.ErrorIs(t, db.First(&user.Addresses).Error, gorm.ErrRecordNotFound) // 倒是会全部删除

		require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
		require.NoError(t, db.Exec("TRUNCATE TABLE addresses").Error)
	}

	// 注意这个移除是 gorm.DB 级别的，不是 gorm.Session 级别的，所以此 test 是独立的 db 实例
	createCallback := db.Callback().Create()
	createCallback.Remove("gorm:save_before_associations")
	createCallback.Remove("gorm:save_after_associations")

	deleteCallback := db.Callback().Delete()
	deleteCallback.Remove("gorm:delete_before_associations")

	updateCallback := db.Callback().Update()
	updateCallback.Remove("gorm:save_before_associations")
	updateCallback.Remove("gorm:save_after_associations")

	// 没准这种方式更完整，是否会有隐患呢？
	afterHandleOmitAssociations(t, db, true)
}

func TestOmitAssociationsByBeforeCallbacks(t *testing.T) {
	env, err := testenv.New().DBEnable(true).SetUp()
	if err != nil {
		panic(err)
	}
	defer env.TearDown()

	db := env.DB
	db.Logger = db.Logger.LogMode(logger.Info)
	db.Config.DisableForeignKeyConstraintWhenMigrating = true

	omitAssociations := func(db *gorm.DB) {
		db.Statement.Omit(clause.Associations)
	}
	// db.Callback().Create().Before("gorm:save_before_associations").Register("omit_associations", omitAssociations)
	// db.Callback().Delete().Before("gorm:delete_before_associations").Register("omit_associations", omitAssociations)
	// db.Callback().Update().Before("gorm:save_before_associations").Register("omit_associations", omitAssociations)

	// 还是推荐这种方案，其实还是自动遵循了 Omit 的方案，比较自然。
	db.Callback().Create().Before("gorm:before_create").Register("omit_associations", omitAssociations)
	db.Callback().Delete().Before("gorm:before_delete").Register("omit_associations", omitAssociations)
	db.Callback().Update().Before("gorm:before_update").Register("omit_associations", omitAssociations)

	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	afterHandleOmitAssociations(t, db, true)
}

// 测试根据 hooks 来进行移除关联数据的写行为
var OmitAssociationsByHooks = false

type Model struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (u *Model) BeforeSave(tx *gorm.DB) error {
	if OmitAssociationsByHooks {
		tx.Statement.Omit(clause.Associations)
	}
	return nil
}

func (u *Model) BeforeDelete(tx *gorm.DB) error {
	if OmitAssociationsByHooks {
		tx.Statement.Omit(clause.Associations)
	}
	return nil
}

func TestOmitAssociationsByHooks(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	OmitAssociationsByHooks = true
	defer func() {
		OmitAssociationsByHooks = false
	}()

	// 这种方式会因为例如 UpdateColumn 之类的操作不会触发 Hooks 而导致处理得不够完整
	// 使用者避免使用 UpdateColumn 之类的方法+FullSaveAssociations 就能避免这个问题
	afterHandleOmitAssociations(t, db, false)
}

func TestOmitAssociationsByScope(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}, &Address{}))

	// 依靠一个 scope+session 来固定 Omit 语句
	db := db.Scopes(func(db *gorm.DB) *gorm.DB {
		return db.Omit(clause.Associations)
	}).Session(&gorm.Session{})

	// 本来以为这样的话 Preload 就不行了呢，没想到也行。
	// 而且本以为对 Delete 来说 .Omit(clause.Associations) 在前/.Select(clause.Associations) 在后 这种情况会认后者呢，没想到也不会
	// 所以貌似和感知上有点不匹配，没准会算是 gorm 以后可能会修复的 bug ？
	afterHandleOmitAssociations(t, db, true)
}
